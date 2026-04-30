package importdata

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"html"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

const (
	maxUploadBytes             = 64 << 20
	legacyDocConversionTimeout = 45 * time.Second
	redAnswerMarker            = "[ÄÃ¡p Ã¡n mÃ u Ä‘á»]"
)

var (
	questionLinePattern           = regexp.MustCompile(`(?i)^\s*(?:c.{0,4}u|cau|question|q)\s*(\d{1,4})\s*[:.)/\]-]*\s*(.*)$`)
	questionMarkerBoundaryPattern = regexp.MustCompile(`(?i)(?:^|\s)(?:c.{0,4}u|cau|question|q)\s*\d{1,4}\s*[:.)/\]-]*`)
	numberedQuestionPattern       = regexp.MustCompile(`^\s*(\d{1,4})\s*[/.)]+\s*(.{1,})$`)
	looseNumberedQuestionPattern  = regexp.MustCompile(`^\s*(\d{1,4})\s+(.{8,})$`)
	decimalFragmentPattern        = regexp.MustCompile(`^\s*\d+[\.,]\d+\b`)
	unnumberedQuestionPattern     = regexp.MustCompile(`(?i)^\s*(?:c.{0,4}u\s*h.{0,4}i|cau\s*hoi|question)\s*[:.)/\]-]*\s*(.*)$`)
	embeddedQuestionPattern       = regexp.MustCompile(`(?i)\s+((?:c.{0,4}u|question|q)\s*\d{1,4}(?:\s*[:.)/\]-]+|\s+).+)$`)
	bareQuestionWordPattern       = regexp.MustCompile(`(?i)^\s*(?:c.{0,4}u|question|q)\s*$`)
	optionLinePattern             = regexp.MustCompile(`^\s*([A-H])\s*[.)\]:-]?\s+(.+)$`)
	inlineOptionMarkerPattern     = regexp.MustCompile(`(?i)(?:^|\s)[A-H]\s*[.)\]:-]\s+`)
	optionLabelOnlyPattern        = regexp.MustCompile(`^\s*([A-H])\s*[.)\]:-]\s*$`)
	lowercaseListLinePattern      = regexp.MustCompile(`^\s*[a-h]\s*[-.)]\s+.+$`)
	imagePlaceholderPattern       = regexp.MustCompile(`(?i)^\s*\[H(?:Ã¬nh|inh)\s+\d+(?:[^\]]*)?\]\s*$`)
	selectOnePattern              = regexp.MustCompile(`(?i)^\s*(?:select\s+one|chá»n\s+má»™t|chon\s+mot)\s*:?$`)
	answerLinePattern             = regexp.MustCompile(`(?i)^\s*(?:đáp\s*án|dap\s*an|Ä‘Ã¡p\s*Ã¡n|đ/a|Ä‘/a|d/a|answer|key)\s*[:.)\]-]?\s*(.+)$`)
	redAnswerLinePattern          = regexp.MustCompile(`(?i)^\s*\[(?:đáp án màu đỏ|Ä‘Ã¡p Ã¡n mÃ u Ä‘á»)\]\s*(.+)$`)
	answerPairPattern             = regexp.MustCompile(`(?i)(?:câu|cÃ¢u|cau)?\s*(\d{1,4})\s*[.)/\]:-]?\s*([A-H])\b`)
	aikenAnswerPattern            = regexp.MustCompile(`(?i)^\s*(?:answer|key|dap\s*an)\s*[:.)\]-]?\s*([A-H])\s*$`)
	singleAnswerPattern           = regexp.MustCompile(`(?i)\b([A-H])\b`)
	pdfPagePattern                = regexp.MustCompile(`/Type\s*/Page\b`)
	pdfImagePattern               = regexp.MustCompile(`/Subtype\s*/Image\b`)
	pdfFontPattern                = regexp.MustCompile(`/Font\b`)
	htmlBreakPattern              = regexp.MustCompile(`(?i)<\s*/?\s*(?:br|p|div|li|tr|td|th|h[1-6])\b[^>]*>`)
	htmlTagPattern                = regexp.MustCompile(`(?s)<[^>]+>`)
	moodleQuestionOrderPattern    = regexp.MustCompile(`(?i)(?:c.{0,4}u|cau|question|q)\s*(\d{1,4})\b`)
	oleDocumentHeader             = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
)

type ParseUploadResult struct {
	ImportBatchID       int64                      `json:"importBatchId,omitempty"`
	RawFile             []byte                     `json:"-"`
	FileSHA256          string                     `json:"fileSha256,omitempty"`
	File                FileInfo                   `json:"file"`
	Extract             ExtractInfo                `json:"extract"`
	Assets              []ExtractedAsset           `json:"-"`
	Questions           []ParsedQuestion           `json:"questions"`
	Summary             ParseSummary               `json:"summary"`
	DuplicateCandidates []ImportDuplicateCandidate `json:"duplicateCandidates,omitempty"`
	Message             string                     `json:"message"`
}

type ImportDuplicateCandidate struct {
	BatchID               int64  `json:"batchId"`
	Title                 string `json:"title"`
	SourceName            string `json:"sourceName"`
	ExistingQuestionCount int    `json:"existingQuestionCount"`
	MatchingQuestionCount int    `json:"matchingQuestionCount"`
	NewQuestionCount      int    `json:"newQuestionCount"`
	CreatedAt             string `json:"createdAt"`
}

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Kind string `json:"kind"`
}

type ExtractInfo struct {
	Status            string   `json:"status"`
	Text              string   `json:"text"`
	NeedsOCR          bool     `json:"needsOcr"`
	NeedsConversion   bool     `json:"needsConversion"`
	Warning           string   `json:"warning"`
	PageEstimate      int      `json:"pageEstimate"`
	ImageCount        int      `json:"imageCount"`
	DocumentTitle     string   `json:"documentTitle"`
	HeadingCandidates []string `json:"headingCandidates"`
}

type ExtractedAsset struct {
	FileName string `json:"fileName"`
	MimeType string `json:"mimeType"`
	Size     int    `json:"size"`
	Data     []byte `json:"-"`
}

type ParsedQuestion struct {
	ImportItemID int64          `json:"importItemId,omitempty"`
	SourceOrder  int            `json:"sourceOrder"`
	Content      string         `json:"content"`
	Options      []ParsedOption `json:"options"`
	CorrectLabel string         `json:"correctLabel,omitempty"`
	Confidence   int            `json:"confidence"`
	Status       string         `json:"status"`
	Warnings     []string       `json:"warnings"`
}

type ParsedOption struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

type ParseSummary struct {
	Total             int `json:"total"`
	Passed            int `json:"passed"`
	Review            int `json:"review"`
	Failed            int `json:"failed"`
	AverageConfidence int `json:"averageConfidence"`
}

type draftQuestion struct {
	sourceOrder     int
	contentParts    []string
	options         []ParsedOption
	correctLabel    string
	expectOptions   bool
	explicitOptions bool
}

func ParseUpload(file multipart.File, header *multipart.FileHeader) (ParseUploadResult, error) {
	if header == nil {
		return ParseUploadResult{}, errors.New("missing file")
	}
	defer file.Close()

	limited := io.LimitReader(file, maxUploadBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return ParseUploadResult{}, err
	}
	if int64(len(data)) > maxUploadBytes {
		return ParseUploadResult{}, errors.New("file is too large")
	}

	kind := sniffFileKind(strings.ToLower(strings.TrimPrefix(filepath.Ext(header.Filename), ".")), data)
	result := ParseUploadResult{
		RawFile:    data,
		FileSHA256: fileSHA256(data),
		File:       FileInfo{Name: header.Filename, Size: header.Size, Kind: kind},
	}

	text, extract, assets := extractContent(kind, data)
	result.Extract = extract
	result.Assets = assets
	if result.Extract.ImageCount == 0 && len(result.Assets) > 0 {
		result.Extract.ImageCount = len(result.Assets)
	}
	if text == "" {
		result.Message = extract.Warning
		result.Questions = []ParsedQuestion{}
		result.Summary = summarize(nil)
		return result, nil
	}

	result.Extract.Text = text
	applyDocumentHeadings(&result.Extract, text)
	result.Questions = ParseText(text)
	result.Summary = summarize(result.Questions)
	result.Message = "Server Ä‘Ã£ tÃ¡ch text vÃ  cháº¡y parser local."
	return result, nil
}

func fileSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ContentFingerprint(text string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func extractContent(kind string, data []byte) (string, ExtractInfo, []ExtractedAsset) {
	if extractor := contentExtractors()[kind]; extractor != nil {
		return extractor(data)
	}
	return "", ExtractInfo{Status: "unsupported", Warning: "Định dạng này chưa được server hỗ trợ ở bước import đầu tiên."}, nil
}
func normalizeExtractedText(kind string, text string, extract ExtractInfo) (string, ExtractInfo) {
	source := strings.TrimSpace(text)
	if source == "" {
		return text, extract
	}

	if looksLikeMoodleXML(source) {
		if converted, info, err := extractMoodleXML([]byte(source)); err == nil && strings.TrimSpace(converted) != "" {
			info.PageEstimate = extract.PageEstimate
			info.ImageCount = extract.ImageCount
			info.NeedsConversion = extract.NeedsConversion
			info.NeedsOCR = extract.NeedsOCR
			info.Warning = joinWarnings(extract.Warning, info.Warning)
			return converted, info
		}
	}

	if converted, info, ok := extractAikenText(source); ok {
		info.PageEstimate = extract.PageEstimate
		info.ImageCount = extract.ImageCount
		info.NeedsConversion = extract.NeedsConversion
		info.NeedsOCR = extract.NeedsOCR
		info.Warning = joinWarnings(extract.Warning, info.Warning)
		return converted, info
	}

	if converted, info, ok := extractGiftText(source); ok {
		info.PageEstimate = extract.PageEstimate
		info.ImageCount = extract.ImageCount
		info.NeedsConversion = extract.NeedsConversion
		info.NeedsOCR = extract.NeedsOCR
		info.Warning = joinWarnings(extract.Warning, info.Warning)
		return converted, info
	}

	extract.Warning = joinWarnings(extract.Warning, formatFallbackWarning(kind))
	return text, extract
}

func formatFallbackWarning(kind string) string {
	switch kind {
	case "doc", "docx", "pdf", "txt":
		return "KhÃ´ng nháº­n diá»‡n chuáº©n Moodle XML, GIFT hoáº·c Aiken; há»‡ thá»‘ng dÃ¹ng rule parser má»m cho ná»™i dung Ä‘Ã£ tÃ¡ch."
	default:
		return ""
	}
}

func joinWarnings(values ...string) string {
	parts := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		parts = append(parts, value)
	}
	return strings.Join(parts, " ")
}

func looksLikeMoodleXML(source string) bool {
	lower := strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(lower, "<quiz") || strings.Contains(lower, "<question") && strings.Contains(lower, "<answer")
}

type normalizedQuestion struct {
	Order        int
	Content      string
	Options      []ParsedOption
	CorrectLabel string
}

func buildNormalizedQuestionText(questions []normalizedQuestion) string {
	var builder strings.Builder
	for index, question := range questions {
		content := strings.Join(strings.Fields(question.Content), " ")
		if content == "" || len(question.Options) == 0 {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		order := question.Order
		if order <= 0 {
			order = index + 1
		}
		builder.WriteString("Cau ")
		builder.WriteString(strconv.Itoa(order))
		builder.WriteString(": ")
		builder.WriteString(content)
		builder.WriteByte('\n')
		for _, option := range question.Options {
			if strings.TrimSpace(option.Label) == "" || strings.TrimSpace(option.Content) == "" {
				continue
			}
			builder.WriteString(strings.ToUpper(option.Label))
			builder.WriteString(". ")
			builder.WriteString(strings.Join(strings.Fields(option.Content), " "))
			builder.WriteByte('\n')
		}
		if question.CorrectLabel != "" {
			builder.WriteString("Dap an: ")
			builder.WriteString(strings.ToUpper(question.CorrectLabel))
			builder.WriteByte('\n')
		}
	}
	return strings.TrimSpace(builder.String())
}

type aikenDraft struct {
	ContentParts []string
	Options      []ParsedOption
	CorrectLabel string
}

func extractAikenText(source string) (string, ExtractInfo, bool) {
	lines := parserLines(source)
	questions := []normalizedQuestion{}
	var current *aikenDraft

	flush := func() {
		if current == nil {
			return
		}
		content := strings.TrimSpace(strings.Join(current.ContentParts, " "))
		if content != "" && len(current.Options) >= 2 && current.CorrectLabel != "" {
			questions = append(questions, normalizedQuestion{
				Order:        len(questions) + 1,
				Content:      content,
				Options:      current.Options,
				CorrectLabel: current.CorrectLabel,
			})
		}
		current = nil
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if matches := aikenAnswerPattern.FindStringSubmatch(line); len(matches) >= 2 {
			if current != nil && len(current.Options) >= 2 {
				if answer := strings.ToUpper(matches[1]); answer != "" {
					current.CorrectLabel = answer
					flush()
					continue
				}
			}
		}
		if matches := optionLinePattern.FindStringSubmatch(line); len(matches) >= 3 {
			if current == nil {
				current = &aikenDraft{}
			}
			current.Options = append(current.Options, ParsedOption{
				Label:   strings.ToUpper(matches[1]),
				Content: strings.TrimSpace(matches[2]),
			})
			continue
		}
		if current != nil && len(current.Options) > 0 {
			last := len(current.Options) - 1
			current.Options[last].Content = strings.TrimSpace(current.Options[last].Content + " " + line)
			continue
		}
		if current == nil {
			current = &aikenDraft{}
		}
		current.ContentParts = append(current.ContentParts, line)
	}
	flush()

	if len(questions) < 2 {
		return "", ExtractInfo{}, false
	}
	text := buildNormalizedQuestionText(questions)
	if text == "" {
		return "", ExtractInfo{}, false
	}
	return text, ExtractInfo{
		Status:        "text_extracted",
		DocumentTitle: "Aiken question bank",
		Warning:       "ÄÃ£ nháº­n diá»‡n Ä‘á»‹nh dáº¡ng Aiken: cÃ¢u há»i, lá»±a chá»n A-D vÃ  dÃ²ng ANSWER/ÄÃ¡p Ã¡n Ä‘Æ°á»£c chuáº©n hÃ³a trÆ°á»›c khi parser local cháº¡y.",
	}, true
}

func firstAnswerLabel(value string) string {
	matches := singleAnswerPattern.FindStringSubmatch(strings.ToUpper(value))
	if len(matches) < 2 {
		return ""
	}
	return strings.ToUpper(matches[1])
}

type giftChoice struct {
	Content string
	Correct bool
}

func extractGiftText(source string) (string, ExtractInfo, bool) {
	blocks := giftBlocks(source)
	if len(blocks) == 0 {
		return "", ExtractInfo{}, false
	}
	questions := []normalizedQuestion{}
	for _, block := range blocks {
		choices := parseGiftChoices(block.Answers)
		if len(choices) < 2 {
			continue
		}
		options := make([]ParsedOption, 0, len(choices))
		correctLabel := ""
		for index, choice := range choices {
			if index >= 8 {
				break
			}
			label := string(rune('A' + index))
			options = append(options, ParsedOption{Label: label, Content: choice.Content})
			if correctLabel == "" && choice.Correct {
				correctLabel = label
			}
		}
		if correctLabel == "" {
			continue
		}
		questions = append(questions, normalizedQuestion{
			Order:        len(questions) + 1,
			Content:      block.Question,
			Options:      options,
			CorrectLabel: correctLabel,
		})
	}
	text := buildNormalizedQuestionText(questions)
	if text == "" {
		return "", ExtractInfo{}, false
	}
	return text, ExtractInfo{
		Status:        "text_extracted",
		DocumentTitle: "GIFT question bank",
		Warning:       "ÄÃ£ nháº­n diá»‡n Ä‘á»‹nh dáº¡ng GIFT: Ä‘Ã¡p Ã¡n '=' vÃ  lá»±a chá»n '~' Ä‘Æ°á»£c chuáº©n hÃ³a trÆ°á»›c khi parser local cháº¡y.",
	}, true
}

type giftBlock struct {
	Question string
	Answers  string
}

func giftBlocks(source string) []giftBlock {
	cleaned := stripGiftComments(strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n"))
	blocks := []giftBlock{}
	lastEnd := 0
	for index := 0; index < len(cleaned); index++ {
		if cleaned[index] != '{' || isEscaped(cleaned, index) {
			continue
		}
		end := matchingBrace(cleaned, index)
		if end <= index {
			continue
		}
		answerBody := strings.TrimSpace(cleaned[index+1 : end])
		if !looksLikeGiftAnswerBlock(answerBody) {
			continue
		}
		question := cleanGiftQuestion(cleaned[lastEnd:index])
		if question != "" {
			blocks = append(blocks, giftBlock{Question: question, Answers: answerBody})
		}
		lastEnd = end + 1
		index = end
	}
	return blocks
}

func stripGiftComments(source string) string {
	lines := strings.Split(source, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func cleanGiftQuestion(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "::") {
		if end := strings.Index(value[2:], "::"); end >= 0 {
			value = strings.TrimSpace(value[end+4:])
		}
	}
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.TrimSpace(strings.Join(cleaned, " "))
}

func matchingBrace(source string, start int) int {
	depth := 0
	for index := start; index < len(source); index++ {
		if isEscaped(source, index) {
			continue
		}
		switch source[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func isEscaped(source string, index int) bool {
	slashes := 0
	for i := index - 1; i >= 0 && source[i] == '\\'; i-- {
		slashes++
	}
	return slashes%2 == 1
}

func looksLikeGiftAnswerBlock(value string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(value))
	if trimmed == "T" || trimmed == "TRUE" || trimmed == "F" || trimmed == "FALSE" {
		return true
	}
	for index := 0; index < len(value); index++ {
		if isEscaped(value, index) {
			continue
		}
		if value[index] == '=' || value[index] == '~' {
			return true
		}
	}
	return false
}

func parseGiftChoices(answerBody string) []giftChoice {
	trimmed := strings.TrimSpace(strings.ToUpper(answerBody))
	switch trimmed {
	case "T", "TRUE":
		return []giftChoice{{Content: "True", Correct: true}, {Content: "False", Correct: false}}
	case "F", "FALSE":
		return []giftChoice{{Content: "True", Correct: false}, {Content: "False", Correct: true}}
	}

	choices := []giftChoice{}
	currentCorrect := false
	var current strings.Builder
	flush := func() {
		content := cleanGiftChoice(current.String())
		if content != "" {
			choices = append(choices, giftChoice{Content: content, Correct: currentCorrect})
		}
		current.Reset()
	}
	for index := 0; index < len(answerBody); index++ {
		ch := answerBody[index]
		if (ch == '=' || ch == '~') && !isEscaped(answerBody, index) {
			flush()
			currentCorrect = ch == '='
			continue
		}
		current.WriteByte(ch)
	}
	flush()
	return choices
}

func cleanGiftChoice(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "%") {
		if end := strings.Index(value[1:], "%"); end >= 0 {
			value = strings.TrimSpace(value[end+2:])
		}
	}
	if cut := strings.Index(value, "#"); cut >= 0 {
		value = value[:cut]
	}
	replacer := strings.NewReplacer(`\=`, "=", `\~`, "~", `\{`, "{", `\}`, "}", `\:`, ":", `\\`, `\`)
	return strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(value))), " ")
}

func sniffFileKind(extension string, data []byte) string {
	if bytes.HasPrefix(data, oleDocumentHeader) {
		return "doc"
	}
	if bytes.HasPrefix(data, []byte{'P', 'K', 0x03, 0x04}) || bytes.HasPrefix(data, []byte{'P', 'K', 0x05, 0x06}) || bytes.HasPrefix(data, []byte{'P', 'K', 0x07, 0x08}) {
		if extension == "doc" || extension == "docx" {
			return "docx"
		}
	}
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		return "pdf"
	}
	return extension
}

func extractLegacyDoc(data []byte, imageCount int) (string, ExtractInfo, []ExtractedAsset) {
	converter, err := findOfficeConverter()
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, err.Error()), scanEmbeddedImageAssets(data)
	}

	workDir, err := os.MkdirTemp("", "exam-doc-convert-*")
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, err.Error()), scanEmbeddedImageAssets(data)
	}
	defer os.RemoveAll(workDir)

	sourcePath := filepath.Join(workDir, "source.doc")
	if err := os.WriteFile(sourcePath, data, 0o600); err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, err.Error()), scanEmbeddedImageAssets(data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), legacyDocConversionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, converter, "--headless", "--convert-to", "docx", "--outdir", workDir, sourcePath)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", needsLegacyDocConversionInfo(imageCount, "LibreOffice convert quÃ¡ thá»i gian cho phÃ©p"), scanEmbeddedImageAssets(data)
	}
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, strings.TrimSpace(string(output))), scanEmbeddedImageAssets(data)
	}

	convertedPath := filepath.Join(workDir, "source.docx")
	converted, err := os.ReadFile(convertedPath)
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, "khÃ´ng tÃ¬m tháº¥y file DOCX sau khi convert: "+err.Error()), scanEmbeddedImageAssets(data)
	}
	text, assets, convertedImages, err := extractDocxPackage(converted)
	if convertedImages > imageCount {
		imageCount = convertedImages
	}
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, "DOC Ä‘Ã£ convert nhÆ°ng chÆ°a tÃ¡ch Ä‘Æ°á»£c text: "+err.Error()), scanEmbeddedImageAssets(data)
	}
	text, extract := normalizeExtractedText("doc", text, ExtractInfo{
		Status:     "text_extracted",
		ImageCount: imageCount,
		Warning:    "DOC cÅ© Ä‘Ã£ Ä‘Æ°á»£c convert sang DOCX báº±ng LibreOffice trÆ°á»›c khi tÃ¡ch text.",
	})
	return text, extract, assets
}

func needsLegacyDocConversionInfo(imageCount int, reason string) ExtractInfo {
	warning := "File DOC cÅ© cáº§n Ä‘Æ°á»£c chuyá»ƒn Ä‘á»•i sang DOCX/TXT báº±ng LibreOffice hoáº·c bá»™ converter server trÆ°á»›c khi parser tÃ¡ch cÃ¢u há»i."
	if reason != "" {
		warning += " LÃ½ do: " + limitMessage(reason, 240) + "."
	}
	if imageCount > 0 {
		warning += " Server Ä‘Ã£ nháº­n diá»‡n áº£nh nhÃºng trong file vÃ  sáº½ cáº§n tÃ¡ch asset riÃªng."
	}
	return ExtractInfo{
		Status:          "needs_conversion",
		NeedsConversion: true,
		ImageCount:      imageCount,
		Warning:         warning,
	}
}

func findOfficeConverter() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("SOFFICE_PATH")); configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured, nil
		}
		return "", errors.New("SOFFICE_PATH khÃ´ng tá»“n táº¡i: " + configured)
	}

	for _, name := range []string{"soffice", "soffice.exe", "soffice.com", "libreoffice", "libreoffice.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	for _, path := range []string{
		`C:\Program Files\LibreOffice\program\soffice.exe`,
		`C:\Program Files\LibreOffice\program\soffice.com`,
		`C:\Program Files (x86)\LibreOffice\program\soffice.exe`,
		`C:\Program Files (x86)\LibreOffice\program\soffice.com`,
	} {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", errors.New("chÆ°a tÃ¬m tháº¥y LibreOffice/soffice trong PATH hoáº·c SOFFICE_PATH")
}

func limitMessage(message string, max int) string {
	message = strings.Join(strings.Fields(message), " ")
	if len(message) <= max {
		return message
	}
	return message[:max]
}

func cleanText(data []byte) string {
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return string(bytes.ToValidUTF8(data, []byte(" ")))
	}
	return string(data)
}

func extractCSVQuestions(data []byte) (string, ExtractInfo, error) {
	source := cleanText(data)
	reader := csv.NewReader(strings.NewReader(source))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = false
	if strings.Count(firstNonEmptyLine(source), ";") > strings.Count(firstNonEmptyLine(source), ",") {
		reader.Comma = ';'
	}
	records, err := reader.ReadAll()
	if err != nil {
		return "", ExtractInfo{}, err
	}
	records = nonEmptyCSVRecords(records)
	if len(records) == 0 {
		return "", ExtractInfo{}, errors.New("file khÃ´ng cÃ³ dÃ²ng dá»¯ liá»‡u")
	}

	header := csvHeaderMap(records[0])
	start := 0
	if _, ok := header["question"]; ok {
		start = 1
	} else if _, ok := header["content"]; ok {
		start = 1
	}
	hasHeader := start == 1

	var builder strings.Builder
	count := 0
	for _, record := range records[start:] {
		questionText := csvCell(record, header, "question", 0)
		if questionText == "" {
			questionText = csvCell(record, header, "content", 0)
		}
		if questionText == "" {
			continue
		}

		count++
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		order := csvCell(record, header, "order", -1)
		if order == "" {
			order = strconv.Itoa(count)
		}
		builder.WriteString("Cau ")
		builder.WriteString(order)
		builder.WriteString(": ")
		builder.WriteString(strings.Join(strings.Fields(questionText), " "))
		builder.WriteByte('\n')

		for optionIndex, label := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
			fallbackIndex := optionIndex + 1
			if hasHeader {
				fallbackIndex = -1
			}
			content := csvCell(record, header, strings.ToLower(label), fallbackIndex)
			if content == "" {
				continue
			}
			builder.WriteString(label)
			builder.WriteString(". ")
			builder.WriteString(strings.Join(strings.Fields(content), " "))
			builder.WriteByte('\n')
		}
		answerFallback := len(record) - 1
		if hasHeader {
			answerFallback = -1
		}
		answer := strings.ToUpper(strings.TrimSpace(csvCell(record, header, "answer", answerFallback)))
		if answer != "" {
			builder.WriteString("Dap an: ")
			builder.WriteString(answer)
			builder.WriteByte('\n')
		}
	}

	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", ExtractInfo{}, errors.New("khÃ´ng tÃ¬m tháº¥y cÃ¢u há»i há»£p lá»‡ trong CSV")
	}
	return text, ExtractInfo{
		Status:        "text_extracted",
		DocumentTitle: "CSV question bank",
		Warning:       "CSV Ä‘Ã£ Ä‘Æ°á»£c Ä‘á»c theo cá»™t question,A,B,C,D,answer trÆ°á»›c khi cháº¡y parser local.",
	}, nil
}

func firstNonEmptyLine(source string) string {
	for _, line := range strings.Split(source, "\n") {
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

func nonEmptyCSVRecords(records [][]string) [][]string {
	cleaned := make([][]string, 0, len(records))
	for _, record := range records {
		hasContent := false
		for _, cell := range record {
			if strings.TrimSpace(cell) != "" {
				hasContent = true
				break
			}
		}
		if hasContent {
			cleaned = append(cleaned, record)
		}
	}
	return cleaned
}

func csvHeaderMap(record []string) map[string]int {
	header := map[string]int{}
	for index, cell := range record {
		key := normalizeCSVHeader(cell)
		if key != "" {
			header[key] = index
		}
	}
	return header
}

func normalizeCSVHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	switch value {
	case "question", "cauhoi", "content", "noidung":
		if value == "content" || value == "noidung" {
			return "content"
		}
		return "question"
	case "a", "b", "c", "d", "e", "f", "g", "h":
		return value
	case "answer", "dapan", "key", "correct":
		return "answer"
	case "order", "stt", "no":
		return "order"
	default:
		return value
	}
}

func csvCell(record []string, header map[string]int, key string, fallbackIndex int) string {
	if index, ok := header[key]; ok && index >= 0 && index < len(record) {
		return strings.TrimSpace(record[index])
	}
	if fallbackIndex >= 0 && fallbackIndex < len(record) {
		return strings.TrimSpace(record[fallbackIndex])
	}
	return ""
}

type moodleQuiz struct {
	Questions []moodleQuestion `xml:"question"`
}

type moodleTextBlock struct {
	Text string `xml:"text"`
}

type moodleQuestion struct {
	Type         string            `xml:"type,attr"`
	Name         moodleTextBlock   `xml:"name"`
	QuestionText moodleTextBlock   `xml:"questiontext"`
	Answers      []moodleAnswer    `xml:"answer"`
	Tags         []moodleTextBlock `xml:"tags>tag"`
}

type moodleAnswer struct {
	Fraction string         `xml:"fraction,attr"`
	Text     moodleTextNode `xml:"text"`
}

type moodleTextNode struct {
	Text string `xml:",chardata"`
}

func extractMoodleXML(data []byte) (string, ExtractInfo, error) {
	var quiz moodleQuiz
	if err := xml.Unmarshal(bytes.TrimSpace(data), &quiz); err != nil {
		return "", ExtractInfo{}, err
	}
	if len(quiz.Questions) == 0 {
		return "", ExtractInfo{}, errors.New("khÃ´ng tÃ¬m tháº¥y question trong XML Moodle")
	}

	var builder strings.Builder
	headings := []string{}
	accepted := 0
	for _, question := range quiz.Questions {
		if question.Type != "" && question.Type != "multichoice" {
			continue
		}

		content := moodleHTMLTextToPlain(question.QuestionText.Text)
		if content == "" {
			continue
		}

		accepted++
		order := accepted
		if parsedOrder := moodleQuestionOrder(question.Name.Text); parsedOrder > 0 {
			order = parsedOrder
		}
		if len(headings) < 4 {
			if title := moodleHTMLTextToPlain(question.Name.Text); title != "" {
				headings = append(headings, title)
			}
		}

		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString("Cau ")
		builder.WriteString(strconv.Itoa(order))
		builder.WriteString(": ")
		builder.WriteString(content)
		builder.WriteByte('\n')

		correctLabel := ""
		optionCount := 0
		for _, answer := range question.Answers {
			optionText := moodleHTMLTextToPlain(answer.Text.Text)
			if optionText == "" {
				continue
			}
			label := string(rune('A' + optionCount))
			optionCount++
			builder.WriteString(label)
			builder.WriteString(". ")
			builder.WriteString(optionText)
			builder.WriteByte('\n')
			if correctLabel == "" && moodleAnswerFraction(answer.Fraction) > 0 {
				correctLabel = label
			}
		}
		if correctLabel != "" {
			builder.WriteString("Dap an: ")
			builder.WriteString(correctLabel)
			builder.WriteByte('\n')
		}
	}

	text := strings.TrimSpace(builder.String())
	if text == "" {
		return "", ExtractInfo{}, errors.New("XML Moodle khÃ´ng cÃ³ cÃ¢u multichoice Ä‘á»c Ä‘Æ°á»£c")
	}

	title := "Moodle XML"
	if len(headings) > 0 {
		title = headings[0]
	}
	return text, ExtractInfo{
		Status:            "text_extracted",
		DocumentTitle:     title,
		HeadingCandidates: headings,
		Warning:           "XML Moodle Ä‘Ã£ Ä‘Æ°á»£c Ä‘á»c theo cáº¥u trÃºc question/answer; cÃ´ng thá»©c LaTeX Ä‘Æ°á»£c giá»¯ nguyÃªn Ä‘á»ƒ frontend render.",
	}, nil
}

func moodleQuestionOrder(value string) int {
	matches := moodleQuestionOrderPattern.FindStringSubmatch(moodleHTMLTextToPlain(value))
	if len(matches) < 2 {
		return 0
	}
	order, _ := strconv.Atoi(matches[1])
	return order
}

func moodleAnswerFraction(value string) float64 {
	fraction, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return fraction
}

func moodleHTMLTextToPlain(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = html.UnescapeString(value)
	value = htmlBreakPattern.ReplaceAllString(value, "\n")
	value = htmlTagPattern.ReplaceAllString(value, " ")
	value = strings.ReplaceAll(value, "\u00a0", " ")
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.TrimSpace(strings.Join(cleaned, " "))
}

func extractDocx(data []byte) (string, error) {
	text, _, _, err := extractDocxPackage(data)
	return text, err
}

func extractDocxPackage(data []byte) (string, []ExtractedAsset, int, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", nil, 0, err
	}
	relationships := readDocxRelationships(reader)
	mediaByPath := readDocxMedia(reader)
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", nil, 0, err
		}
		defer rc.Close()
		text, assets, count, err := extractDocxXML(rc, relationships, mediaByPath)
		return text, assets, count, err
	}
	return "", nil, 0, errors.New("word/document.xml not found")
}

func readDocxRelationships(reader *zip.Reader) map[string]string {
	relationships := map[string]string{}
	for _, file := range reader.File {
		if file.Name != "word/_rels/document.xml.rels" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return relationships
		}
		defer rc.Close()
		decoder := xml.NewDecoder(rc)
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return relationships
			}
			element, ok := token.(xml.StartElement)
			if !ok || element.Name.Local != "Relationship" {
				continue
			}
			var id, target string
			for _, attr := range element.Attr {
				switch attr.Name.Local {
				case "Id":
					id = attr.Value
				case "Target":
					target = attr.Value
				}
			}
			if id == "" || target == "" {
				continue
			}
			if strings.HasPrefix(target, "/") {
				target = strings.TrimPrefix(target, "/")
			} else {
				target = path.Clean(path.Join("word", target))
			}
			relationships[id] = target
		}
		break
	}
	return relationships
}

func readDocxMedia(reader *zip.Reader) map[string]ExtractedAsset {
	media := map[string]ExtractedAsset{}
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "word/media/") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxUploadBytes))
		rc.Close()
		if err != nil || len(content) == 0 {
			continue
		}
		media[path.Clean(file.Name)] = ExtractedAsset{
			FileName: filepath.Base(file.Name),
			MimeType: mimeTypeForAsset(file.Name, content),
			Size:     len(content),
			Data:     content,
		}
	}
	return media
}

func extractDocxXML(reader io.Reader, relationships map[string]string, mediaByPath map[string]ExtractedAsset) (string, []ExtractedAsset, int, error) {
	decoder := xml.NewDecoder(reader)
	var builder strings.Builder
	var paragraph strings.Builder
	inText := false
	inParagraph := false
	inRun := false
	runRed := false
	paragraphDefaultRed := false
	paragraphRed := false
	imageCount := 0
	orderedAssets := []ExtractedAsset{}
	flushParagraph := func() {
		text := strings.Join(strings.Fields(paragraph.String()), " ")
		if text != "" {
			builder.WriteString(text)
			builder.WriteByte('\n')
			if paragraphRed {
				builder.WriteString(redAnswerMarker)
				builder.WriteByte(' ')
				builder.WriteString(text)
				builder.WriteByte('\n')
			}
		}
		paragraph.Reset()
		paragraphDefaultRed = false
		paragraphRed = false
	}
	appendImagePlaceholder := func(relID string) {
		imageCount++
		if target := relationships[relID]; target != "" {
			if asset, ok := mediaByPath[path.Clean(target)]; ok {
				orderedAssets = append(orderedAssets, asset)
			}
		}
		paragraph.WriteString(" [HÃ¬nh ")
		paragraph.WriteString(strconv.Itoa(imageCount))
		paragraph.WriteString("] ")
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", orderedAssets, imageCount, err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local == "p" {
				inParagraph = true
				paragraph.Reset()
				paragraphDefaultRed = false
				paragraphRed = false
			}
			if typed.Name.Local == "r" {
				inRun = true
				runRed = paragraphDefaultRed
			}
			if typed.Name.Local == "color" {
				red := isRedWordColor(typed)
				if inRun {
					runRed = red
				} else if inParagraph && red {
					paragraphDefaultRed = true
				}
			}
			if typed.Name.Local == "t" {
				inText = true
			}
			if typed.Name.Local == "blip" || typed.Name.Local == "imagedata" {
				for _, attr := range typed.Attr {
					if attr.Name.Local == "embed" || attr.Name.Local == "link" || attr.Name.Local == "id" {
						appendImagePlaceholder(attr.Value)
						break
					}
				}
			}
			if typed.Name.Local == "br" {
				flushParagraph()
			}
			if typed.Name.Local == "tab" {
				paragraph.WriteByte(' ')
			}
		case xml.EndElement:
			if typed.Name.Local == "t" {
				inText = false
			}
			if typed.Name.Local == "r" {
				inRun = false
				runRed = false
			}
			if typed.Name.Local == "p" {
				flushParagraph()
				inParagraph = false
			}
		case xml.CharData:
			if inText {
				text := string(typed)
				paragraph.WriteString(text)
				if runRed && strings.TrimSpace(text) != "" {
					paragraphRed = true
				}
			}
		}
	}
	if paragraph.Len() > 0 {
		flushParagraph()
	}
	if len(orderedAssets) == 0 && imageCount > 0 {
		for _, asset := range mediaByPath {
			orderedAssets = append(orderedAssets, asset)
		}
	}
	return builder.String(), orderedAssets, imageCount, nil
}

func isRedWordColor(element xml.StartElement) bool {
	for _, attr := range element.Attr {
		if attr.Name.Local != "val" {
			continue
		}
		value := strings.ToUpper(strings.TrimPrefix(attr.Value, "#"))
		return value == "FF0000" || value == "C00000" || value == "E60000" || strings.HasPrefix(value, "FF")
	}
	return false
}

func extractPDF(data []byte) (string, error) {
	tmp, err := os.CreateTemp("", "exam-import-*.pdf")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	file, reader, err := pdf.Open(tmpPath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	buffer, err := reader.GetPlainText()
	if err != nil {
		return "", err
	}
	text, err := io.ReadAll(buffer)
	if err != nil {
		return "", err
	}
	return string(text), nil
}

func inspectPDF(data []byte) (pages int, images int, fonts int) {
	return len(pdfPagePattern.FindAll(data, -1)),
		len(pdfImagePattern.FindAll(data, -1)),
		len(pdfFontPattern.FindAll(data, -1))
}

func inspectDocxMedia(data []byte) int {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return 0
	}
	count := 0
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "word/media/") {
			count++
		}
	}
	return count
}

func inspectEmbeddedImages(data []byte) int {
	return bytes.Count(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}) +
		bytes.Count(data, []byte{0xFF, 0xD8, 0xFF}) +
		bytes.Count(data, []byte{'G', 'I', 'F', '8'})
}

func extractAssets(kind string, data []byte) []ExtractedAsset {
	switch kind {
	case "docx":
		return extractDocxAssets(data)
	case "doc":
		return scanEmbeddedImageAssets(data)
	default:
		return nil
	}
}

func extractDocxAssets(data []byte) []ExtractedAsset {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	assets := []ExtractedAsset{}
	for _, file := range reader.File {
		if !strings.HasPrefix(file.Name, "word/media/") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxUploadBytes))
		rc.Close()
		if err != nil || len(content) == 0 {
			continue
		}
		assets = append(assets, ExtractedAsset{
			FileName: filepath.Base(file.Name),
			MimeType: mimeTypeForAsset(file.Name, content),
			Size:     len(content),
			Data:     content,
		})
	}
	return assets
}

func scanEmbeddedImageAssets(data []byte) []ExtractedAsset {
	assets := []ExtractedAsset{}
	assets = append(assets, scanImageSignature(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'}, []byte{'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82}, "png", "image/png")...)
	assets = append(assets, scanImageSignature(data, []byte{0xFF, 0xD8, 0xFF}, []byte{0xFF, 0xD9}, "jpg", "image/jpeg")...)
	for index := range assets {
		assets[index].FileName = "embedded-" + leftPadInt(index+1, 3) + filepath.Ext(assets[index].FileName)
	}
	return assets
}

func scanImageSignature(data []byte, startSignature []byte, endSignature []byte, ext string, mimeType string) []ExtractedAsset {
	assets := []ExtractedAsset{}
	offset := 0
	for {
		start := bytes.Index(data[offset:], startSignature)
		if start < 0 {
			break
		}
		start += offset
		end := bytes.Index(data[start+len(startSignature):], endSignature)
		if end < 0 {
			break
		}
		end += start + len(startSignature) + len(endSignature)
		content := append([]byte(nil), data[start:end]...)
		assets = append(assets, ExtractedAsset{
			FileName: "embedded." + ext,
			MimeType: mimeType,
			Size:     len(content),
			Data:     content,
		})
		offset = end
	}
	return assets
}

func mimeTypeForAsset(name string, data []byte) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	}
	if bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) {
		return "image/png"
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return "image/jpeg"
	}
	return "application/octet-stream"
}

func leftPadInt(value int, width int) string {
	text := strconv.Itoa(value)
	if len(text) >= width {
		return text
	}
	return strings.Repeat("0", width-len(text)) + text
}

func applyDocumentHeadings(extract *ExtractInfo, text string) {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	headings := make([]string, 0, 5)
	for _, raw := range lines {
		line := strings.Join(strings.Fields(raw), " ")
		if line == "" {
			continue
		}
		if questionLinePattern.MatchString(line) || numberedQuestionPattern.MatchString(line) {
			break
		}
		if utf8.RuneCountInString(line) >= 6 {
			headings = append(headings, line)
			if len(headings) == 5 {
				break
			}
		}
	}
	if len(headings) == 0 {
		return
	}
	extract.HeadingCandidates = headings
	extract.DocumentTitle = normalizeDocumentTitle(headings)
}

func normalizeDocumentTitle(headings []string) string {
	for _, heading := range headings {
		candidate := strings.Trim(strings.Join(strings.Fields(heading), " "), " :-")
		if candidate == "" || numberedQuestionPattern.MatchString(candidate) || looseNumberedQuestionPattern.MatchString(candidate) {
			continue
		}
		if utf8.RuneCountInString(candidate) > 80 {
			candidate = firstWords(candidate, 10)
		}
		return candidate
	}
	return ""
}

func firstWords(text string, maxWords int) string {
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return strings.Join(words, " ")
	}
	return strings.Join(words[:maxWords], " ")
}

func ParseText(source string) []ParsedQuestion {
	lines := parserLines(source)
	var drafts []draftQuestion
	answerMap := map[int]string{}
	var current *draftQuestion
	fallbackOrder := 1

	pushCurrent := func() {
		if current == nil {
			return
		}
		trimDraft(current)
		if len(current.contentParts) > 0 || len(current.options) > 0 {
			drafts = append(drafts, *current)
		}
	}

	for _, line := range lines {
		if line == "" {
			continue
		}
		if shouldSkipParserLine(line) {
			continue
		}
		if selectOnePattern.MatchString(line) {
			if current != nil {
				current.expectOptions = true
			}
			continue
		}
		if isImagePlaceholderLine(line) {
			if current != nil {
				appendVisualLine(current, line)
			}
			continue
		}

		if redAnswer := redAnswerLinePattern.FindStringSubmatch(line); redAnswer != nil {
			applyStyledAnswer(current, redAnswer[1])
			continue
		}

		if answer := answerLinePattern.FindStringSubmatch(line); answer != nil {
			collectAnswerPairs(answer[1], answerMap)
			if current != nil {
				if single := singleAnswerPattern.FindStringSubmatch(answer[1]); single != nil {
					current.correctLabel = strings.ToUpper(single[1])
				}
			}
			continue
		}

		var question []string
		if !looksLikeAnswerList(line) && !looksLikeDecimalFragment(line) {
			question = questionLinePattern.FindStringSubmatch(line)
			if question == nil {
				question = numberedQuestionPattern.FindStringSubmatch(line)
			}
			if question == nil {
				question = looseNumberedQuestionPattern.FindStringSubmatch(line)
			}
		}
		if question != nil {
			parsedOrder, validOrder := atoi(question[1])
			if !validOrder || parsedOrder <= 0 {
				question = nil
			}
		}
		if question != nil {
			pushCurrent()
			order, _ := atoi(question[1])
			current = &draftQuestion{
				sourceOrder:   order,
				contentParts:  nonEmpty(question[2]),
				options:       []ParsedOption{},
				expectOptions: strings.HasSuffix(strings.TrimSpace(question[2]), "?"),
			}
			fallbackOrder = order + 1
			continue
		}

		if questionText := unnumberedQuestionPattern.FindStringSubmatch(line); questionText != nil && strings.TrimSpace(questionText[1]) != "" {
			pushCurrent()
			current = &draftQuestion{
				sourceOrder:   fallbackOrder,
				contentParts:  nonEmpty(questionText[1]),
				options:       []ParsedOption{},
				expectOptions: true,
			}
			fallbackOrder++
			continue
		}

		if looksLikeLooseQuestion(line, current) {
			pushCurrent()
			current = &draftQuestion{
				sourceOrder:   fallbackOrder,
				contentParts:  nonEmpty(line),
				options:       []ParsedOption{},
				expectOptions: true,
			}
			fallbackOrder++
			continue
		}

		if current != nil {
			if option := optionLinePattern.FindStringSubmatch(line); option != nil {
				current.options = append(current.options, ParsedOption{
					Label:   strings.ToUpper(option[1]),
					Content: strings.TrimSpace(option[2]),
				})
				current.expectOptions = true
				current.explicitOptions = true
				continue
			}
			if option := optionLabelOnlyPattern.FindStringSubmatch(line); option != nil {
				current.options = append(current.options, ParsedOption{
					Label:   strings.ToUpper(option[1]),
					Content: "",
				})
				current.expectOptions = true
				current.explicitOptions = true
				continue
			}
			if shouldInferUnlabeledOption(current, line) {
				current.options = append(current.options, ParsedOption{
					Label:   optionLabel(len(current.options)),
					Content: line,
				})
				continue
			}
		}

		if current == nil {
			continue
		}
		if len(current.options) > 0 {
			last := &current.options[len(current.options)-1]
			last.Content = strings.TrimSpace(last.Content + " " + line)
		} else {
			current.contentParts = append(current.contentParts, line)
		}
	}
	pushCurrent()

	questions := make([]ParsedQuestion, 0, len(drafts))
	for _, draft := range drafts {
		if draft.correctLabel == "" {
			draft.correctLabel = answerMap[draft.sourceOrder]
		}
		questions = append(questions, scoreQuestion(draft))
	}
	return applyCrossQuestionWarnings(questions)
}

func looksLikeDecimalFragment(line string) bool {
	return decimalFragmentPattern.MatchString(line)
}

func atoi(value string) (int, bool) {
	n := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
		n = n*10 + int(char-'0')
	}
	return n, true
}

func parserLines(source string) []string {
	rawLines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(source, "\r\n", "\n"), "\r", "\n"), "\n")
	normalized := make([]string, 0, len(rawLines))
	for _, raw := range rawLines {
		line := strings.Join(strings.Fields(raw), " ")
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}

	lines := make([]string, 0, len(normalized))
	for index := 0; index < len(normalized); index++ {
		line := normalized[index]
		if bareQuestionWordPattern.MatchString(line) && index+1 < len(normalized) {
			if _, ok := atoi(strings.Trim(normalized[index+1], " :.)/]-")); ok {
				line = line + " " + normalized[index+1]
				index++
			}
		}
		for _, questionSegment := range splitEmbeddedQuestionLine(line) {
			lines = append(lines, splitInlineOptionLine(questionSegment)...)
		}
	}
	return lines
}

func splitEmbeddedQuestionLine(line string) []string {
	if segments := splitQuestionMarkerSegments(line); len(segments) > 0 {
		return segments
	}
	if questionLinePattern.MatchString(line) || numberedQuestionPattern.MatchString(line) || looseNumberedQuestionPattern.MatchString(line) {
		return []string{line}
	}
	match := embeddedQuestionPattern.FindStringSubmatchIndex(line)
	if match == nil || match[0] <= 0 || len(match) < 4 {
		return []string{line}
	}
	prefix := strings.TrimSpace(line[:match[0]])
	suffix := strings.TrimSpace(line[match[2]:match[3]])
	if prefix == "" || suffix == "" {
		return []string{line}
	}
	return []string{prefix, suffix}
}

func splitQuestionMarkerSegments(line string) []string {
	matches := questionMarkerBoundaryPattern.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return nil
	}
	if len(matches) == 1 && matches[0][0] == 0 {
		return nil
	}

	segments := make([]string, 0, len(matches)+1)
	if matches[0][0] > 0 {
		if prefix := strings.TrimSpace(line[:matches[0][0]]); prefix != "" {
			segments = append(segments, prefix)
		}
	}
	for index, match := range matches {
		end := len(line)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		if segment := strings.TrimSpace(line[match[0]:end]); segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) <= 1 {
		return nil
	}
	return segments
}

func splitInlineOptionLine(line string) []string {
	matches := inlineOptionMarkerPattern.FindAllStringIndex(line, -1)
	if len(matches) == 0 {
		return []string{line}
	}
	if len(matches) == 1 && matches[0][0] == 0 {
		return []string{line}
	}

	segments := make([]string, 0, len(matches)+1)
	if matches[0][0] > 0 {
		if prefix := strings.TrimSpace(line[:matches[0][0]]); prefix != "" {
			segments = append(segments, prefix)
		}
	}
	for index, match := range matches {
		end := len(line)
		if index+1 < len(matches) {
			end = matches[index+1][0]
		}
		if segment := strings.TrimSpace(line[match[0]:end]); segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return []string{line}
	}
	return segments
}

func isImagePlaceholderLine(line string) bool {
	if imagePlaceholderPattern.MatchString(line) {
		return true
	}
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(lower, "[h") && strings.HasSuffix(lower, "]") && strings.Contains(lower, "nh ")
}

func nonEmpty(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

func shouldSkipParserLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) >= 5 && strings.Count(trimmed, "-") == len(trimmed) {
		return true
	}
	lower := strings.ToLower(trimmed)
	return trimmed == ":" || trimmed == "." || lower == "Ä‘oáº¡n vÄƒn cÃ¢u há»i" || lower == "doan van cau hoi"
}

func shouldInferUnlabeledOption(current *draftQuestion, line string) bool {
	if current == nil || len(current.options) >= 8 || lowercaseListLinePattern.MatchString(line) {
		return false
	}
	if current.explicitOptions {
		return false
	}
	if len(current.options) > 0 {
		last := strings.TrimSpace(current.options[len(current.options)-1].Content)
		if last == "" {
			return false
		}
		return current.expectOptions && len(current.options) < 4 && utf8.RuneCountInString(line) <= 120
	}
	if current.expectOptions && utf8.RuneCountInString(line) <= 120 {
		return true
	}
	return false
}

func looksLikeLooseQuestion(line string, current *draftQuestion) bool {
	if !strings.HasSuffix(strings.TrimSpace(line), "?") {
		return false
	}
	return current == nil || len(current.options) >= 4
}

func appendVisualLine(current *draftQuestion, line string) {
	if current == nil {
		return
	}
	current.contentParts = append(current.contentParts, line)
	current.expectOptions = true
}

func applyStyledAnswer(current *draftQuestion, answerText string) {
	if current == nil {
		return
	}
	answerText = strings.TrimSpace(answerText)
	if answerText == "" {
		return
	}
	if option := optionLinePattern.FindStringSubmatch(answerText); option != nil {
		current.correctLabel = strings.ToUpper(option[1])
		return
	}
	needle := normalizeAnswerText(answerText)
	for _, option := range current.options {
		if normalizeAnswerText(option.Content) == needle {
			current.correctLabel = option.Label
			return
		}
	}
}

func normalizeAnswerText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func optionLabel(index int) string {
	if index < 0 || index >= 26 {
		return "?"
	}
	return string(rune('A' + index))
}

func trimDraft(draft *draftQuestion) {
	content := draft.contentParts[:0]
	for _, part := range draft.contentParts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			content = append(content, trimmed)
		}
	}
	draft.contentParts = content

	options := draft.options[:0]
	for _, option := range draft.options {
		option.Content = strings.TrimSpace(option.Content)
		if option.Content != "" {
			options = append(options, option)
		}
	}
	draft.options = options
}

func collectAnswerPairs(line string, answerMap map[int]string) {
	for _, match := range answerPairPattern.FindAllStringSubmatch(line, -1) {
		if order, ok := atoi(match[1]); ok {
			answerMap[order] = strings.ToUpper(match[2])
		}
	}
}

func looksLikeAnswerList(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "Ä‘Ã¡p Ã¡n") || strings.Contains(lower, "dap an") || strings.Contains(lower, "answer")
}

func scoreQuestion(draft draftQuestion) ParsedQuestion {
	content := strings.Join(draft.contentParts, " ")
	content = strings.Join(strings.Fields(content), " ")
	warnings := []string{}
	confidence := 0

	if utf8.RuneCountInString(content) >= 12 {
		confidence += 25
	} else {
		warnings = append(warnings, "Ná»™i dung cÃ¢u há»i quÃ¡ ngáº¯n hoáº·c bá»‹ tÃ¡ch sai.")
	}

	binaryChoice := isBinaryChoiceQuestion(draft.options)
	if len(draft.options) >= 4 {
		confidence += 30
		if len(draft.options) > 4 {
			warnings = append(warnings, "CÃ³ hÆ¡n 4 lá»±a chá»n, cáº§n giÃ¡o viÃªn xÃ¡c nháº­n cÃ¢u nÃ y khÃ´ng bá»‹ dÃ­nh thÃªm dÃ²ng.")
		}
	} else if binaryChoice {
		confidence += 30
	} else if len(draft.options) >= 2 {
		confidence += 18
		warnings = append(warnings, "Ãt hÆ¡n 4 lá»±a chá»n, cáº§n kiá»ƒm tra.")
	} else {
		warnings = append(warnings, "KhÃ´ng Ä‘á»§ lá»±a chá»n A/B/C/D.")
	}

	seen := map[string]bool{}
	duplicate := false
	for _, option := range draft.options {
		if seen[option.Label] {
			duplicate = true
		}
		seen[option.Label] = true
	}
	if !duplicate {
		confidence += 15
	} else {
		warnings = append(warnings, "CÃ³ lá»±a chá»n bá»‹ trÃ¹ng kÃ½ tá»±.")
	}

	usefulOptions := len(draft.options) > 0
	for _, option := range draft.options {
		if utf8.RuneCountInString(option.Content) < 2 {
			usefulOptions = false
			break
		}
	}
	if usefulOptions {
		confidence += 10
	} else {
		warnings = append(warnings, "CÃ³ lá»±a chá»n quÃ¡ ngáº¯n hoáº·c rá»—ng.")
	}

	hasValidAnswer := false
	if draft.correctLabel != "" && seen[draft.correctLabel] {
		confidence += 20
		hasValidAnswer = true
	} else if draft.correctLabel != "" {
		warnings = append(warnings, "ÄÃ¡p Ã¡n "+draft.correctLabel+" khÃ´ng khá»›p lá»±a chá»n Ä‘Ã£ tÃ¡ch.")
	} else {
		warnings = append(warnings, "ChÆ°a tÃ¬m tháº¥y Ä‘Ã¡p Ã¡n Ä‘Ãºng.")
	}

	if !hasValidAnswer && confidence >= 80 {
		confidence = 79
	}
	if (len(draft.options) > 4 || duplicate) && confidence >= 60 {
		confidence = 59
	}

	status := "fail"
	if confidence >= 80 {
		status = "pass"
	} else if confidence >= 60 {
		status = "review"
	}

	return ParsedQuestion{
		SourceOrder:  draft.sourceOrder,
		Content:      content,
		Options:      draft.options,
		CorrectLabel: draft.correctLabel,
		Confidence:   confidence,
		Status:       status,
		Warnings:     warnings,
	}
}

func applyCrossQuestionWarnings(questions []ParsedQuestion) []ParsedQuestion {
	counts := map[string]int{}
	for _, question := range questions {
		counts[duplicateQuestionKey(question)]++
	}
	for index := range questions {
		if counts[duplicateQuestionKey(questions[index])] <= 1 {
			continue
		}
		if questions[index].Confidence >= 80 {
			questions[index].Confidence = 79
		}
		if questions[index].Confidence >= 60 {
			questions[index].Status = "review"
		} else {
			questions[index].Status = "fail"
		}
		questions[index].Warnings = append(questions[index].Warnings, "TrÃ¹ng sá»‘ cÃ¢u vÃ  ná»™i dung vá»›i má»™t cÃ¢u khÃ¡c. KhÃ´ng tá»± xoÃ¡ Ä‘á»ƒ trÃ¡nh máº¥t cÃ¢u tháº­t; giÃ¡o viÃªn cáº§n gá»™p, Ä‘á»•i sá»‘, hoáº·c xoÃ¡ báº£n thá»«a.")
	}
	return questions
}

func isBinaryChoiceQuestion(options []ParsedOption) bool {
	if len(options) != 2 {
		return false
	}
	joined := normalizeAnswerText(options[0].Content + " " + options[1].Content)
	return (strings.Contains(joined, "dung") && strings.Contains(joined, "sai")) ||
		(strings.Contains(joined, "Ä‘Ãºng") && strings.Contains(joined, "sai")) ||
		(strings.Contains(joined, "true") && strings.Contains(joined, "false"))
}

func duplicateQuestionKey(question ParsedQuestion) string {
	return strconv.Itoa(question.SourceOrder) + "|" + normalizeAnswerText(question.Content)
}

func QuestionContentKey(content string) string {
	return normalizeAnswerText(content)
}

func summarize(questions []ParsedQuestion) ParseSummary {
	summary := ParseSummary{Total: len(questions)}
	totalConfidence := 0
	for _, question := range questions {
		totalConfidence += question.Confidence
		switch question.Status {
		case "pass":
			summary.Passed++
		case "review":
			summary.Review++
		default:
			summary.Failed++
		}
	}
	if len(questions) > 0 {
		summary.AverageConfidence = totalConfidence / len(questions)
	}
	return summary
}
