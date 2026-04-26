package importdata

import (
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
