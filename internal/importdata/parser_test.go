package importdata

import (
	"strings"
	"testing"
)

func TestParseTextAllowsLooseQuestionAndOptionFormat(t *testing.T) {
	text := `Câu 1 Thiết bị nào dùng để lưu trữ dữ liệu lâu dài?
A RAM
B. Ổ cứng
C) CPU
D: Màn hình
Đáp án B

2. Hệ điều hành có vai trò gì?
A Quản lý tài nguyên máy tính
B Soạn thảo văn bản
C Thiết kế ảnh
D Tăng tốc mạng
Đáp án: A`

	questions := ParseText(text)
	if len(questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(questions))
	}
	for _, question := range questions {
		if question.Status != "pass" {
			t.Fatalf("expected question %d to pass, got %s warnings=%v", question.SourceOrder, question.Status, question.Warnings)
		}
		if len(question.Options) != 4 {
			t.Fatalf("expected question %d to have 4 options, got %d", question.SourceOrder, len(question.Options))
		}
		if question.CorrectLabel == "" {
			t.Fatalf("expected question %d to have a correct label", question.SourceOrder)
		}
	}
}

func TestParseTextHandlesWordExportNoiseAndUnlabeledOptions(t *testing.T) {
	text := `ĐỀ CƯƠNG XỬ LÝ ẢNH
1/ Ảnh "đen- trắng" là ảnh có
Hai mức chói "0" và "1"
Các điểm ảnh với mức xám khác 0
Nhiều mức xám nằm trong khoảng Lmin-Lmax
Độ bão hoà màu bằng 0

2/ Các giai đoạn chính trong xử lý ảnh
a- Thu nhận hình ảnh
b- trích chọn dấu hiệu
c- Tiền xử lý ảnh
d- hậu xử lý
Hãy cho biết thứ tự đúng của các giai đoạn là
A. abcdef
B. abedfc
C. acbefd
D. cabdfe

Câu hỏi: Trong MATLAB, để tạo bộ lọc trung bình 3x3 cho việc lọc làm mịn ảnh chúng ta không thể dùng phương pháp nào sau đây ?
Select one:
h = ones(3)/9
h = fspecial('average',[3 3])
h = fspecial('average', 3)
h = ones(3)/3`

	questions := ParseText(text)
	if len(questions) != 3 {
		t.Fatalf("expected 3 questions, got %d: %#v", len(questions), questions)
	}
	for _, question := range questions {
		if len(question.Options) < 4 {
			t.Fatalf("expected question %d to have at least 4 options, got %d: %#v", question.SourceOrder, len(question.Options), question.Options)
		}
	}
	if questions[0].Options[0].Label != "A" || questions[0].Options[3].Label != "D" {
		t.Fatalf("expected inferred labels A-D, got %#v", questions[0].Options)
	}
	if !strings.Contains(questions[1].Content, "Thu nhận") {
		t.Fatalf("expected lowercase a-/b- list to stay in question content, got %q", questions[1].Content)
	}
	if questions[0].Status == "pass" {
		t.Fatalf("expected missing answer key to require review, got pass")
	}
}

func TestParseTextKeepsImagePlaceholderInQuestionContent(t *testing.T) {
	text := `33/ Các điểm ảnh lân cận dạng nào có trong block 9 điểm ảnh trên hình vẽ:
[Hình 1]
Chỉ có lân cận dạng N4
Chỉ có lân cận dạng ND
Chỉ có lân cận dạng N8
Lân cận N4NDN8`

	questions := ParseText(text)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if !strings.Contains(questions[0].Content, "[Hình 1]") {
		t.Fatalf("expected image placeholder in content, got %q", questions[0].Content)
	}
	if len(questions[0].Options) != 4 {
		t.Fatalf("expected 4 options, got %#v", questions[0].Options)
	}
}

func TestParseTextFlagsDuplicateQuestionNumberAndExtraOptions(t *testing.T) {
	text := `Cau 1 Noi dung cau hoi thu nhat co du do dai?
A. Mot
B. Hai
C. Ba
D. Bon
Dap an A

Cau 1 Noi dung cau hoi thu nhat co du do dai?
A. Mot
B. Hai
C. Ba
D. Bon
Dap an B

Cau 2 Noi dung cau hoi co nam lua chon?
A. Mot
B. Hai
C. Ba
D. Bon
E. Nam
Dap an C`

	questions := ParseText(text)
	if len(questions) != 3 {
		t.Fatalf("expected 3 questions, got %d", len(questions))
	}
	if questions[0].Status != "review" || questions[1].Status != "review" {
		t.Fatalf("expected duplicate question numbers to require review, got %s and %s", questions[0].Status, questions[1].Status)
	}
	if questions[2].Status != "review" {
		t.Fatalf("expected question with extra option to require review, got %s warnings=%v", questions[2].Status, questions[2].Warnings)
	}
}

func TestParseTextHandlesUnnumberedQuestionAndStyledAnswerMarker(t *testing.T) {
	text := `Đối tượng nghiên cứu của môn học Tư tưởng Hồ Chí Minh là gì?
Cuộc đời, sự nghiệp Hồ Chí Minh
Tiểu sử Hồ Chí Minh
Lịch sử tư tưởng Hồ Chí Minh
Hệ thống các quan điểm lý luận của Hồ Chí Minh
[Đáp án màu đỏ] Hệ thống các quan điểm lý luận của Hồ Chí Minh`

	questions := ParseText(text)
	if len(questions) != 1 {
		t.Fatalf("expected 1 question, got %d", len(questions))
	}
	if questions[0].CorrectLabel != "D" {
		t.Fatalf("expected red answer to map to D, got %q question=%#v", questions[0].CorrectLabel, questions[0])
	}
	if questions[0].Status != "pass" {
		t.Fatalf("expected styled answer question to pass, got %s warnings=%v", questions[0].Status, questions[0].Warnings)
	}
}

func TestExtractTextDetectsLegacyDocWithImages(t *testing.T) {
	data := append([]byte{}, oleDocumentHeader...)
	data = append(data, []byte("noise")...)
	data = append(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n', 'I', 'E', 'N', 'D', 0xAE, 0x42, 0x60, 0x82}...)
	data = append(data, []byte{0xFF, 0xD8, 0xFF, 0x00, 0xFF, 0xD9}...)

	text, extract := extractText("doc", data)
	if text != "" {
		t.Fatalf("expected no text for legacy doc, got %q", text)
	}
	if !extract.NeedsConversion || extract.Status != "needs_conversion" {
		t.Fatalf("expected needs_conversion, got status=%s needs=%v", extract.Status, extract.NeedsConversion)
	}
	if extract.ImageCount != 2 {
		t.Fatalf("expected 2 embedded images, got %d", extract.ImageCount)
	}
	assets := extractAssets("doc", data)
	if len(assets) != 2 {
		t.Fatalf("expected 2 extracted assets, got %d", len(assets))
	}
}

func TestSniffFileKindDoesNotTrustWrongDocxExtension(t *testing.T) {
	data := append([]byte{}, oleDocumentHeader...)
	if kind := sniffFileKind("docx", data); kind != "doc" {
		t.Fatalf("expected legacy OLE content to be treated as doc, got %s", kind)
	}
	if kind := sniffFileKind("doc", []byte{'P', 'K', 0x03, 0x04}); kind != "docx" {
		t.Fatalf("expected zip content to be treated as docx, got %s", kind)
	}
}

func TestFindOfficeConverterUsesConfiguredPath(t *testing.T) {
	t.Setenv("SOFFICE_PATH", "missing-soffice.exe")
	if _, err := findOfficeConverter(); err == nil {
		t.Fatal("expected invalid SOFFICE_PATH to fail")
	}
}

func TestApplyDocumentHeadingsKeepsTitleBeforeQuestions(t *testing.T) {
	extract := ExtractInfo{}
	applyDocumentHeadings(&extract, "De cuong tin hoc dai cuong\nHoc ky 1\nCau 1 Noi dung cau hoi?\nA. Mot")
	if extract.DocumentTitle != "De cuong tin hoc dai cuong" {
		t.Fatalf("unexpected title %q", extract.DocumentTitle)
	}
	if len(extract.HeadingCandidates) != 2 {
		t.Fatalf("expected 2 heading candidates, got %d", len(extract.HeadingCandidates))
	}
}

func TestExtractDocxXMLKeepsImagePlaceholder(t *testing.T) {
	source := `<document><body><p><r><t>81/ Cho ma trận A, B.</t></r><r><drawing/></r></p><p><r><t>A. Z</t></r></p></body></document>`
	text, err := extractDocxXML(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "[Hình 1]") {
		t.Fatalf("expected image placeholder, got %q", text)
	}
}

func TestExtractDocxXMLMarksRedParagraphAsAnswer(t *testing.T) {
	source := `<document><body><p><r><t>Đối tượng nghiên cứu là gì?</t></r></p><p><r><t>A. Sai</t></r></p><p><r><rPr><color val="FF0000"/></rPr><t>B. Đúng</t></r></p></body></document>`
	text, err := extractDocxXML(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "[Đáp án màu đỏ] B. Đúng") {
		t.Fatalf("expected red answer marker, got %q", text)
	}
}

func TestNormalizeDocumentTitleSkipsQuestionLines(t *testing.T) {
	title := normalizeDocumentTitle([]string{"1/ Ảnh đen trắng là gì", "ĐỀ CƯƠNG XỬ LÝ ẢNH"})
	if title != "ĐỀ CƯƠNG XỬ LÝ ẢNH" {
		t.Fatalf("unexpected title %q", title)
	}
}
