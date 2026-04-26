package importdata

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"os/exec"
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
	redAnswerMarker            = "[Đáp án màu đỏ]"
)

var (
	questionLinePattern          = regexp.MustCompile(`(?i)^\s*(?:c.{0,4}u|cau|question|q)\s*(\d{1,4})\s*[:.)/\]-]*\s*(.*)$`)
	numberedQuestionPattern      = regexp.MustCompile(`^\s*(\d{1,4})\s*[/.)]+\s*(.{1,})$`)
	looseNumberedQuestionPattern = regexp.MustCompile(`^\s*(\d{1,4})\s+(.{8,})$`)
	decimalFragmentPattern       = regexp.MustCompile(`^\s*\d+[\.,]\d+\b`)
	unnumberedQuestionPattern    = regexp.MustCompile(`(?i)^\s*(?:c.{0,4}u\s*h.{0,4}i|cau\s*hoi|question)\s*[:.)/\]-]*\s*(.*)$`)
	embeddedQuestionPattern      = regexp.MustCompile(`(?i)\s+((?:c.{0,4}u|question|q)\s*\d{1,4}(?:\s*[:.)/\]-]+|\s+).+)$`)
	bareQuestionWordPattern      = regexp.MustCompile(`(?i)^\s*(?:c.{0,4}u|question|q)\s*$`)
	optionLinePattern            = regexp.MustCompile(`^\s*([A-H])\s*[.)\]:-]?\s+(.+)$`)
	optionLabelOnlyPattern       = regexp.MustCompile(`^\s*([A-H])\s*[.)\]:-]\s*$`)
	lowercaseListLinePattern     = regexp.MustCompile(`^\s*[a-h]\s*[-.)]\s+.+$`)
	imagePlaceholderPattern      = regexp.MustCompile(`^\s*\[Hình\s+\d+\]\s*$`)
	selectOnePattern             = regexp.MustCompile(`(?i)^\s*(?:select\s+one|chọn\s+một|chon\s+mot)\s*:?$`)
	answerLinePattern            = regexp.MustCompile(`(?i)^\s*(?:đáp\s*án|dap\s*an|đ/a|d/a|answer|key)\s*[:.)\]-]?\s*(.+)$`)
	redAnswerLinePattern         = regexp.MustCompile(`(?i)^\s*\[đáp án màu đỏ\]\s*(.+)$`)
	answerPairPattern            = regexp.MustCompile(`(?i)(?:câu|cau)?\s*(\d{1,4})\s*[.)/\]:-]?\s*([A-H])\b`)
	singleAnswerPattern          = regexp.MustCompile(`(?i)\b([A-H])\b`)
	pdfPagePattern               = regexp.MustCompile(`/Type\s*/Page\b`)
	pdfImagePattern              = regexp.MustCompile(`/Subtype\s*/Image\b`)
	pdfFontPattern               = regexp.MustCompile(`/Font\b`)
	oleDocumentHeader            = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
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

	text, extract := extractText(kind, data)
	result.Extract = extract
	result.Assets = extractAssets(kind, data)
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
	result.Message = "Server đã tách text và chạy parser local."
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

func extractText(kind string, data []byte) (string, ExtractInfo) {
	switch kind {
	case "txt", "csv":
		return cleanText(data), ExtractInfo{Status: "text_extracted"}
	case "docx":
		images := inspectDocxMedia(data)
		text, err := extractDocx(data)
		if err != nil {
			return "", ExtractInfo{Status: "failed", Warning: "DOCX chưa tách được nội dung: " + err.Error()}
		}
		return text, ExtractInfo{Status: "text_extracted", ImageCount: images}
	case "doc":
		images := inspectEmbeddedImages(data)
		if bytes.HasPrefix(data, oleDocumentHeader) {
			return extractLegacyDoc(data, images)
		}
		return "", ExtractInfo{Status: "unsupported", ImageCount: images, Warning: "File .doc này không đúng header OLE cũ nên server chưa có bộ đọc phù hợp."}
	case "pdf":
		text, err := extractPDF(data)
		pages, images, fonts := inspectPDF(data)
		if err == nil && strings.TrimSpace(text) != "" {
			return text, ExtractInfo{Status: "text_extracted", PageEstimate: pages, ImageCount: images}
		}
		if images > 0 && fonts == 0 {
			return "", ExtractInfo{
				Status:       "needs_ocr",
				NeedsOCR:     true,
				ImageCount:   images,
				PageEstimate: pages,
				Warning:      "PDF này giống dạng scan ảnh nên cần OCR trước khi parser local chạy được.",
			}
		}
		return "", ExtractInfo{Status: "failed", PageEstimate: pages, Warning: "PDF chưa tách được text. Cần thêm OCR hoặc bộ extract PDF mạnh hơn."}
	default:
		return "", ExtractInfo{Status: "unsupported", Warning: "Định dạng này chưa được server hỗ trợ ở bước import đầu tiên."}
	}
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

func extractLegacyDoc(data []byte, imageCount int) (string, ExtractInfo) {
	converter, err := findOfficeConverter()
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, err.Error())
	}

	workDir, err := os.MkdirTemp("", "exam-doc-convert-*")
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, err.Error())
	}
	defer os.RemoveAll(workDir)

	sourcePath := filepath.Join(workDir, "source.doc")
	if err := os.WriteFile(sourcePath, data, 0o600); err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), legacyDocConversionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, converter, "--headless", "--convert-to", "docx", "--outdir", workDir, sourcePath)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", needsLegacyDocConversionInfo(imageCount, "LibreOffice convert quá thời gian cho phép")
	}
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, strings.TrimSpace(string(output)))
	}

	convertedPath := filepath.Join(workDir, "source.docx")
	converted, err := os.ReadFile(convertedPath)
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, "không tìm thấy file DOCX sau khi convert: "+err.Error())
	}
	text, err := extractDocx(converted)
	convertedImages := inspectDocxMedia(converted)
	if convertedImages > imageCount {
		imageCount = convertedImages
	}
	if err != nil {
		return "", needsLegacyDocConversionInfo(imageCount, "DOC đã convert nhưng chưa tách được text: "+err.Error())
	}
	return text, ExtractInfo{
		Status:     "text_extracted",
		ImageCount: imageCount,
		Warning:    "DOC cũ đã được convert sang DOCX bằng LibreOffice trước khi tách text.",
	}
}

func needsLegacyDocConversionInfo(imageCount int, reason string) ExtractInfo {
	warning := "File DOC cũ cần được chuyển đổi sang DOCX/TXT bằng LibreOffice hoặc bộ converter server trước khi parser tách câu hỏi."
	if reason != "" {
		warning += " Lý do: " + limitMessage(reason, 240) + "."
	}
	if imageCount > 0 {
		warning += " Server đã nhận diện ảnh nhúng trong file và sẽ cần tách asset riêng."
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
		return "", errors.New("SOFFICE_PATH không tồn tại: " + configured)
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

	return "", errors.New("chưa tìm thấy LibreOffice/soffice trong PATH hoặc SOFFICE_PATH")
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

func extractDocx(data []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return extractDocxXML(rc)
	}
	return "", errors.New("word/document.xml not found")
}

func extractDocxXML(reader io.Reader) (string, error) {
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
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
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
			if typed.Name.Local == "drawing" || typed.Name.Local == "pict" {
				imageCount++
				paragraph.WriteString(" [Hình ")
				paragraph.WriteString(strconv.Itoa(imageCount))
				paragraph.WriteString("] ")
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
	return builder.String(), nil
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
		lines = append(lines, splitEmbeddedQuestionLine(line)...)
	}
	return lines
}

func splitEmbeddedQuestionLine(line string) []string {
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
	return trimmed == ":" || trimmed == "." || lower == "đoạn văn câu hỏi" || lower == "doan van cau hoi"
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
	return strings.Contains(lower, "đáp án") || strings.Contains(lower, "dap an") || strings.Contains(lower, "answer")
}

func scoreQuestion(draft draftQuestion) ParsedQuestion {
	content := strings.Join(draft.contentParts, " ")
	content = strings.Join(strings.Fields(content), " ")
	warnings := []string{}
	confidence := 0

	if utf8.RuneCountInString(content) >= 12 {
		confidence += 25
	} else {
		warnings = append(warnings, "Nội dung câu hỏi quá ngắn hoặc bị tách sai.")
	}

	if len(draft.options) >= 4 {
		confidence += 30
		if len(draft.options) > 4 {
			warnings = append(warnings, "Có hơn 4 lựa chọn, cần giáo viên xác nhận câu này không bị dính thêm dòng.")
		}
	} else if len(draft.options) >= 2 {
		confidence += 18
		warnings = append(warnings, "Ít hơn 4 lựa chọn, cần kiểm tra.")
	} else {
		warnings = append(warnings, "Không đủ lựa chọn A/B/C/D.")
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
		warnings = append(warnings, "Có lựa chọn bị trùng ký tự.")
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
		warnings = append(warnings, "Có lựa chọn quá ngắn hoặc rỗng.")
	}

	hasValidAnswer := false
	if draft.correctLabel != "" && seen[draft.correctLabel] {
		confidence += 20
		hasValidAnswer = true
	} else if draft.correctLabel != "" {
		warnings = append(warnings, "Đáp án "+draft.correctLabel+" không khớp lựa chọn đã tách.")
	} else {
		warnings = append(warnings, "Chưa tìm thấy đáp án đúng.")
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
		questions[index].Warnings = append(questions[index].Warnings, "Trùng số câu và nội dung với một câu khác. Không tự xoá để tránh mất câu thật; giáo viên cần gộp, đổi số, hoặc xoá bản thừa.")
	}
	return questions
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
