package importdata

import (
	"archive/zip"
	"bytes"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func parseFixtureUpload(t *testing.T, relativePath string) ParseUploadResult {
	t.Helper()
	filePath := filepath.Join("..", "..", relativePath)
	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open fixture %s: %v", relativePath, err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		t.Fatalf("stat fixture %s: %v", relativePath, err)
	}
	result, err := ParseUpload(file, &multipart.FileHeader{
		Filename: filepath.Base(filePath),
		Size:     info.Size(),
	})
	if err != nil {
		t.Fatalf("parse fixture %s: %v", relativePath, err)
	}
	return result
}

func TestParseTextSplitsEmbeddedQuestionMarker(t *testing.T) {
	source := `
Cau 9: Chu ky may cua vi dieu khien 8051 xap xi la:
A. 1.0 us
B. 1.085 us
C. 2.0 us
D. 0.5 us . Cau 10: Lenh nao dung de di chuyen du lieu tu bo nho chuong trinh?
A. MOVC
B. MOV
C. XCH
D. ADD
Dap an: 9 B
Dap an: 10 A
`

	questions := ParseText(source)
	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d: %#v", len(questions), questions)
	}
	if questions[0].SourceOrder != 9 || len(questions[0].Options) != 4 {
		t.Fatalf("question 9 was not kept intact: %#v", questions[0])
	}
	if questions[1].SourceOrder != 10 || questions[1].Content == "" {
		t.Fatalf("question 10 was not split from previous option: %#v", questions[1])
	}
}

func TestParseTextFailsDuplicateOrTooManyOptions(t *testing.T) {
	source := `
Cau 11: 0592 MHz
A. chu ky
B. may cua vi dieu khien 8051 xap xi la
C. xi la
A. 1.0
E. us
F. .
B. 1.085
H. us
C. 2.0 us
D. 0.5 us
Dap an: A
`

	questions := ParseText(source)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].Status != "fail" {
		t.Fatalf("bad table/pdf extraction must fail, got status=%s confidence=%d warnings=%v", questions[0].Status, questions[0].Confidence, questions[0].Warnings)
	}
}

func TestParseTextKeepsStandaloneImageWithQuestion(t *testing.T) {
	source := `
Cau 1: Anh nao dung?
[Hinh 1]
A. Mot
B. Hai
C. Ba
D. Bon
Dap an: A
`

	questions := ParseText(source)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if got := questions[0].Content; got == "" || !strings.Contains(got, "[Hinh 1]") {
		t.Fatalf("standalone image placeholder should stay in question content, got %q", got)
	}
	for _, option := range questions[0].Options {
		if strings.Contains(option.Content, "[Hinh 1]") {
			t.Fatalf("standalone image placeholder was attached to option: %#v", option)
		}
	}
}

func TestParseTextKeepsPDFQuestionBodyBeforeLabelOnlyOptions(t *testing.T) {
	source := `
Câu 1
:
Cổng P0 của vi điều khiển 8051 cần có điện trở kéo lên bên ngoài khi dùng làm
cổng xuất/nhập.
A.
Đúng
B.
Sai
Đáp án: A
`

	questions := ParseText(source)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d: %#v", len(questions), questions)
	}
	got := questions[0]
	if !strings.Contains(got.Content, "Cổng P0") {
		t.Fatalf("question body was parsed as an option: %#v", got)
	}
	if len(got.Options) != 2 || got.Options[0].Content != "Đúng" || got.Options[1].Content != "Sai" {
		t.Fatalf("label-only options were not stitched with following lines: %#v", got.Options)
	}
	if got.CorrectLabel != "A" {
		t.Fatalf("expected answer A, got %q", got.CorrectLabel)
	}
}

func TestParseTextKeepsWrappedExplicitOptionContent(t *testing.T) {
	source := `
Câu 6: Nếu tần số thạch anh là …… thì thời gian thực hiện lệnh NOP là ........
A. 24 MHz,
0.5
B. 12 MHz,
0.5
C. 24 MHz, 2
D. 12 MHz, 2
Đáp án: B
`

	questions := ParseText(source)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d: %#v", len(questions), questions)
	}
	got := questions[0]
	if len(got.Options) != 4 {
		t.Fatalf("wrapped explicit options should stay at 4 choices, got %#v", got.Options)
	}
	if !strings.Contains(got.Options[0].Content, "0.5") || !strings.Contains(got.Options[1].Content, "0.5") {
		t.Fatalf("wrapped option continuation was not appended: %#v", got.Options)
	}
}

func TestParseTextPassesBinaryTrueFalseQuestion(t *testing.T) {
	source := `
Cau 1: Cong P0 cua 8051 can tro keo len ngoai khi dung lam cong xuat nhap.
A. Dung
B. Sai
Dap an: A
`

	questions := ParseText(source)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].Status != "pass" {
		t.Fatalf("binary true/false question should pass, got status=%s confidence=%d warnings=%v", questions[0].Status, questions[0].Confidence, questions[0].Warnings)
	}
}

func TestParseUploadDocxOrdersAssetsByImageOccurrence(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	writeZipFile(t, archive, "word/document.xml", `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"
 xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
 xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
 <w:body>
  <w:p><w:r><w:t>Cau 1: Cau hoi co anh thu hai truoc?</w:t></w:r><w:r><w:drawing><a:blip r:embed="rId2"/></w:drawing></w:r></w:p>
  <w:p><w:r><w:t>A. Dung</w:t></w:r></w:p>
  <w:p><w:r><w:t>B. Sai</w:t></w:r></w:p>
  <w:p><w:r><w:t>Dap an: A</w:t></w:r></w:p>
  <w:p><w:r><w:t>Cau 2: Cau hoi co anh thu nhat?</w:t></w:r><w:r><w:drawing><a:blip r:embed="rId1"/></w:drawing></w:r></w:p>
  <w:p><w:r><w:t>A. Dung</w:t></w:r></w:p>
  <w:p><w:r><w:t>B. Sai</w:t></w:r></w:p>
  <w:p><w:r><w:t>Dap an: B</w:t></w:r></w:p>
 </w:body>
</w:document>`)
	writeZipFile(t, archive, "word/_rels/document.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
 <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>
 <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image2.png"/>
</Relationships>`)
	writeZipFile(t, archive, "word/media/image1.png", string([]byte{0x89, 'P', 'N', 'G', 1}))
	writeZipFile(t, archive, "word/media/image2.png", string([]byte{0x89, 'P', 'N', 'G', 2}))
	if err := archive.Close(); err != nil {
		t.Fatalf("close docx archive: %v", err)
	}

	result, err := ParseUpload(readSeekCloser{Reader: bytes.NewReader(buffer.Bytes())}, &multipart.FileHeader{Filename: "fixture.docx", Size: int64(buffer.Len())})
	if err != nil {
		t.Fatalf("parse docx fixture: %v", err)
	}
	if result.Extract.ImageCount != 2 || len(result.Assets) != 2 {
		t.Fatalf("expected two ordered images, imageCount=%d assets=%#v", result.Extract.ImageCount, result.Assets)
	}
	if result.Assets[0].FileName != "image2.png" || result.Assets[1].FileName != "image1.png" {
		t.Fatalf("assets should follow document occurrence order, got %#v", result.Assets)
	}
	if len(result.Questions) != 2 || !strings.Contains(result.Questions[0].Content, "[H") || !strings.Contains(result.Questions[1].Content, "[H") {
		t.Fatalf("expected image placeholders in both questions, got %#v", result.Questions)
	}
}

func writeZipFile(t *testing.T, archive *zip.Writer, name string, content string) {
	t.Helper()
	writer, err := archive.Create(name)
	if err != nil {
		t.Fatalf("create zip file %s: %v", name, err)
	}
	if _, err := writer.Write([]byte(content)); err != nil {
		t.Fatalf("write zip file %s: %v", name, err)
	}
}

type readSeekCloser struct {
	*bytes.Reader
}

func (reader readSeekCloser) Close() error {
	return nil
}

func TestParseUploadTutuongDocxDoesNotMergeAllQuestions(t *testing.T) {
	result := parseFixtureUpload(t, filepath.Join("input_test", "tutuong.docx"))
	if result.Summary.Total < 100 {
		t.Fatalf("expected many questions from tu tuong outline, got summary=%#v first=%#v", result.Summary, result.Questions)
	}
	if len(result.Questions) == 0 {
		t.Fatalf("expected parsed questions")
	}
	first := result.Questions[0]
	if first.SourceOrder != 1 {
		t.Fatalf("expected first question order 1, got %#v", first)
	}
	if strings.Contains(first.Content, "Hồ Chí Minh đã từng dạy học") || strings.Contains(first.Content, "Hồ Chí Minh ra đi tìm đường") {
		t.Fatalf("first question swallowed later questions: %q", first.Content)
	}
	if len(first.Options) != 4 {
		t.Fatalf("expected first question to have 4 options, got %#v", first.Options)
	}
}

func TestParseUploadSchoolELearningFormDocx(t *testing.T) {
	result := parseFixtureUpload(t, filepath.Join("input_test", "FORM_DE_E_LEARNING.docx"))
	if result.Extract.Status != "text_extracted" {
		t.Fatalf("expected school form DOCX to extract text, got extract=%#v", result.Extract)
	}
	if result.Summary.Total == 0 {
		t.Fatalf("expected at least one parsed question from school form, got summary=%#v warning=%q", result.Summary, result.Extract.Warning)
	}
	if result.Extract.ImageCount > 0 && len(result.Assets) == 0 {
		t.Fatalf("image placeholders need ordered assets, imageCount=%d assets=%d", result.Extract.ImageCount, len(result.Assets))
	}
}

func TestParseUploadPDFDoesNotMergeNextQuestionIntoOption(t *testing.T) {
	result := parseFixtureUpload(t, filepath.Join("input_test", "test_image.pdf"))
	if result.Summary.Total < 7 {
		t.Fatalf("expected at least 7 questions, got summary=%#v questions=%#v", result.Summary, result.Questions)
	}
	var nopQuestion *ParsedQuestion
	var movQuestion *ParsedQuestion
	for index := range result.Questions {
		question := &result.Questions[index]
		if strings.Contains(question.Content, "NOP") {
			nopQuestion = question
		}
		if strings.Contains(question.Content, "MOV A") && strings.Contains(question.Content, "INC A") {
			movQuestion = question
		}
	}
	if nopQuestion == nil || movQuestion == nil {
		t.Fatalf("expected separate NOP and MOV/INC questions, got questions=%#v", result.Questions)
	}
	if nopQuestion.SourceOrder != 6 || movQuestion.SourceOrder != 7 {
		t.Fatalf("expected NOP question 6 and MOV/INC question 7, got q%d and q%d", nopQuestion.SourceOrder, movQuestion.SourceOrder)
	}
	for _, option := range nopQuestion.Options {
		if strings.Contains(option.Content, "Câu 7") || strings.Contains(option.Content, "MOV A") {
			t.Fatalf("question 6 option swallowed question 7: %#v", option)
		}
	}
	if len(nopQuestion.Options) != 4 || len(movQuestion.Options) != 4 {
		t.Fatalf("expected both questions to have 4 options, got q6=%#v q7=%#v", nopQuestion.Options, movQuestion.Options)
	}
	if movQuestion.CorrectLabel == "H" {
		t.Fatalf("hex constants must not be mistaken for answer key H: %#v", movQuestion)
	}
}

func TestParseUploadMoodleXMLPreservesLatexAndAnswers(t *testing.T) {
	result := parseFixtureUpload(t, filepath.Join("input_test", "quiz_XML_De A3 (du phong).xml"))
	if result.File.Kind != "xml" {
		t.Fatalf("expected xml kind, got %q", result.File.Kind)
	}
	if result.Extract.Status != "text_extracted" {
		t.Fatalf("expected Moodle XML text extraction, got %#v", result.Extract)
	}
	if result.Summary.Total != 10 {
		t.Fatalf("expected 10 Moodle questions, got summary=%#v", result.Summary)
	}
	if result.Summary.Passed != 10 {
		t.Fatalf("expected all Moodle questions to pass, got summary=%#v questions=%#v", result.Summary, result.Questions)
	}
	first := result.Questions[0]
	if !strings.Contains(first.Content, `\(`) || !strings.Contains(first.Content, `M_{3\times2}`) {
		t.Fatalf("LaTeX content was not preserved in first question: %q", first.Content)
	}
	if len(first.Options) != 4 || first.CorrectLabel == "" {
		t.Fatalf("expected Moodle options and answer, got %#v", first)
	}
	foundCases := false
	for _, question := range result.Questions {
		if strings.Contains(question.Content, `\begin{cases}`) {
			foundCases = true
			break
		}
		for _, option := range question.Options {
			if strings.Contains(option.Content, `\begin{cases}`) {
				foundCases = true
				break
			}
		}
	}
	if !foundCases {
		t.Fatalf("expected piecewise LaTeX formula to be preserved, got questions=%#v", result.Questions)
	}
}

func TestParseUploadStructuredCSV(t *testing.T) {
	source := `question,A,B,C,D,answer
"Thiết bị nào lưu trữ dữ liệu lâu dài?","RAM","Ổ cứng","CPU","Màn hình",B
"Công thức \(2+2\) bằng bao nhiêu?","3","4","5","6",B`

	result, err := ParseUpload(readSeekCloser{Reader: bytes.NewReader([]byte(source))}, &multipart.FileHeader{
		Filename: "questions.csv",
		Size:     int64(len(source)),
	})
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if result.File.Kind != "csv" {
		t.Fatalf("expected csv kind, got %q", result.File.Kind)
	}
	if result.Summary.Total != 2 || result.Summary.Passed != 2 {
		t.Fatalf("expected two passing CSV questions, got summary=%#v questions=%#v", result.Summary, result.Questions)
	}
	if result.Questions[0].CorrectLabel != "B" || result.Questions[1].CorrectLabel != "B" {
		t.Fatalf("CSV answer column was not mapped: %#v", result.Questions)
	}
	if !strings.Contains(result.Questions[1].Content, `\(2+2\)`) {
		t.Fatalf("CSV LaTeX content should be preserved: %#v", result.Questions[1])
	}
}

func TestParseUploadAikenFormat(t *testing.T) {
	source := `Thiết bị nào lưu dữ liệu lâu dài?
A. RAM
B. Ổ cứng
C. CPU
D. Màn hình
ANSWER: B

Công thức \(2+2\) bằng bao nhiêu?
A. 3
B. 4
C. 5
D. 6
ANSWER: B`

	result, err := ParseUpload(readSeekCloser{Reader: bytes.NewReader([]byte(source))}, &multipart.FileHeader{
		Filename: "questions-aiken.txt",
		Size:     int64(len(source)),
	})
	if err != nil {
		t.Fatalf("parse aiken: %v", err)
	}
	if result.Summary.Total != 2 || result.Summary.Passed != 2 {
		t.Fatalf("expected two passing Aiken questions, got summary=%#v questions=%#v", result.Summary, result.Questions)
	}
	if result.Questions[0].CorrectLabel != "B" || result.Questions[1].CorrectLabel != "B" {
		t.Fatalf("Aiken answer lines were not mapped: %#v", result.Questions)
	}
	if !strings.Contains(result.Extract.DocumentTitle, "Aiken") {
		t.Fatalf("expected Aiken adapter metadata, got extract=%#v", result.Extract)
	}
}

func TestParseUploadGiftFormat(t *testing.T) {
	source := `::Luu tru::Thiết bị nào lưu dữ liệu lâu dài? {
~RAM
=Ổ cứng
~CPU
~Màn hình
}

Công thức \(2+2\) bằng bao nhiêu? {~3 =4 ~5 ~6}`

	result, err := ParseUpload(readSeekCloser{Reader: bytes.NewReader([]byte(source))}, &multipart.FileHeader{
		Filename: "questions-gift.txt",
		Size:     int64(len(source)),
	})
	if err != nil {
		t.Fatalf("parse gift: %v", err)
	}
	if result.Summary.Total != 2 || result.Summary.Passed != 2 {
		t.Fatalf("expected two passing GIFT questions, got summary=%#v questions=%#v", result.Summary, result.Questions)
	}
	if result.Questions[0].CorrectLabel != "B" || result.Questions[1].CorrectLabel != "B" {
		t.Fatalf("GIFT correct choices were not mapped: %#v", result.Questions)
	}
	if !strings.Contains(result.Questions[1].Content, `\(2+2\)`) {
		t.Fatalf("GIFT LaTeX content should be preserved: %#v", result.Questions[1])
	}
}

func TestPublicImportSamplesParse(t *testing.T) {
	fixtures := []string{
		filepath.Join("frontend", "public", "import-samples", "examhub-sample.txt"),
		filepath.Join("frontend", "public", "import-samples", "examhub-sample.csv"),
		filepath.Join("frontend", "public", "import-samples", "examhub-aiken-sample.txt"),
		filepath.Join("frontend", "public", "import-samples", "examhub-gift-sample.txt"),
		filepath.Join("frontend", "public", "import-samples", "examhub-moodle-sample.xml"),
		filepath.Join("frontend", "public", "import-samples", "examhub-sample.docx"),
	}
	for _, fixture := range fixtures {
		result := parseFixtureUpload(t, fixture)
		if result.Summary.Total == 0 {
			t.Fatalf("sample %s should parse at least one question, got summary=%#v extract=%#v", fixture, result.Summary, result.Extract)
		}
		if result.Summary.Passed == 0 {
			t.Fatalf("sample %s should have passing questions, got summary=%#v questions=%#v", fixture, result.Summary, result.Questions)
		}
	}
}
