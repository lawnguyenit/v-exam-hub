package studentdata

type Dashboard struct {
	Profile        Profile         `json:"profile"`
	Summary        Summary         `json:"summary"`
	AvailableExams []ExamSummary   `json:"availableExams"`
	PlannedExams   []PlannedExam   `json:"plannedExams"`
	History        []HistoryRecord `json:"history"`
}

type Profile struct {
	DisplayName string `json:"displayName"`
	ClassName   string `json:"className"`
	Email       string `json:"email"`
	Status      string `json:"status"`
}

type Summary struct {
	AvailableCount int     `json:"availableCount"`
	PlannedCount   int     `json:"plannedCount"`
	LatestScore    float64 `json:"latestScore"`
}

type ExamSummary struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Title    string `json:"title"`
	Meta     string `json:"meta"`
	Duration string `json:"duration"`
}

type PlannedExam struct {
	Time   string `json:"time"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type HistoryRecord struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Date     string  `json:"date"`
	Score    float64 `json:"score"`
	Duration string  `json:"duration"`
}

type Exam struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	DurationSeconds int        `json:"durationSeconds"`
	Questions       []Question `json:"questions"`
}

type Question struct {
	Title         string   `json:"title"`
	Answers       []string `json:"answers"`
	CorrectAnswer int      `json:"correctAnswer"`
}

type Review struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Score     float64          `json:"score"`
	Duration  string           `json:"duration"`
	Questions []ReviewQuestion `json:"questions"`
}

type ReviewQuestion struct {
	Title          string   `json:"title"`
	Answers        []string `json:"answers"`
	CorrectAnswer  int      `json:"correctAnswer"`
	SelectedAnswer int      `json:"selectedAnswer"`
}

func DashboardFor(account string) Dashboard {
	if account == "" {
		account = "UI-DEMO"
	}

	return Dashboard{
		Profile: Profile{
			DisplayName: displayName(account),
			ClassName:   "CNTT K48",
			Email:       "student@school.edu.vn",
			Status:      "Được phép dự thi",
		},
		Summary: Summary{
			AvailableCount: 2,
			PlannedCount:   3,
			LatestScore:    8.2,
		},
		AvailableExams: []ExamSummary{
			{ID: "go-basics-demo", Status: "Đang mở", Title: "Lập trình cơ sở với Go", Meta: "12 câu, tự động lưu tiến trình.", Duration: "38 phút"},
			{ID: "network-practice", Status: "Thi thử", Title: "Kiến thức mạng máy tính", Meta: "20 câu, không tính vào điểm chính thức.", Duration: "45 phút"},
		},
		PlannedExams: []PlannedExam{
			{Time: "15/04 - 08:00", Title: "Cơ sở dữ liệu", Detail: "Chờ đến giờ mở bài"},
			{Time: "18/04 - 13:30", Title: "Cấu trúc dữ liệu", Detail: "Thi chính thức"},
			{Time: "22/04 - 09:00", Title: "An toàn thông tin", Detail: "Thi thử"},
		},
		History: []HistoryRecord{
			{ID: "go-intro", Title: "Nhập môn Go", Date: "10/04/2026", Score: 8.2, Duration: "31 phút"},
			{ID: "html-css", Title: "HTML/CSS cơ bản", Date: "04/04/2026", Score: 7.6, Duration: "26 phút"},
		},
	}
}

func ExamByID(id string) (Exam, bool) {
	switch id {
	case "go-basics-demo":
		return Exam{
			ID:              "go-basics-demo",
			Title:           "Lập trình cơ sở với Go",
			DurationSeconds: 38*60 + 42,
			Questions: []Question{
				{Title: "Goroutine được dùng để làm gì trong Go?", Answers: []string{"Chạy tác vụ đồng thời với chi phí nhẹ", "Biên dịch mã nguồn thành CSS", "Tạo cơ sở dữ liệu quan hệ", "Đóng gói template HTML"}, CorrectAnswer: 0},
				{Title: "Kênh channel trong Go giúp xử lý việc nào?", Answers: []string{"Trao đổi dữ liệu giữa các goroutine", "Vẽ biểu đồ thống kê điểm", "Nén ảnh trước khi upload", "Tạo file CSS tự động"}, CorrectAnswer: 0},
				{Title: "Lệnh nào thường dùng để chạy ứng dụng Go cục bộ?", Answers: []string{"go run .", "go serve ui", "npm publish", "docker style"}, CorrectAnswer: 0},
			},
		}, true
	case "network-practice":
		return Exam{
			ID:              "network-practice",
			Title:           "Kiến thức mạng máy tính",
			DurationSeconds: 45 * 60,
			Questions: []Question{
				{Title: "Giao thức nào thường dùng để phân giải tên miền?", Answers: []string{"DNS", "FTP", "SMTP", "SSH"}, CorrectAnswer: 0},
				{Title: "HTTP status 404 thường biểu thị điều gì?", Answers: []string{"Thành công", "Không tìm thấy tài nguyên", "Lỗi máy chủ", "Chưa xác thực"}, CorrectAnswer: 1},
			},
		}, true
	default:
		return Exam{}, false
	}
}

func ReviewByID(id string) (Review, bool) {
	switch id {
	case "go-intro":
		return Review{
			ID:       "go-intro",
			Title:    "Nhập môn Go",
			Score:    8.2,
			Duration: "31 phút",
			Questions: []ReviewQuestion{
				{Title: "Goroutine được dùng để làm gì trong Go?", Answers: []string{"Chạy tác vụ đồng thời với chi phí nhẹ", "Biên dịch mã nguồn thành CSS", "Tạo cơ sở dữ liệu quan hệ", "Đóng gói template HTML"}, CorrectAnswer: 0, SelectedAnswer: 1},
				{Title: "Kênh channel trong Go giúp xử lý việc nào?", Answers: []string{"Trao đổi dữ liệu giữa các goroutine", "Vẽ biểu đồ thống kê điểm", "Nén ảnh trước khi upload", "Tạo file CSS tự động"}, CorrectAnswer: 0, SelectedAnswer: 0},
			},
		}, true
	case "html-css":
		return Review{
			ID:       "html-css",
			Title:    "HTML/CSS cơ bản",
			Score:    7.6,
			Duration: "26 phút",
			Questions: []ReviewQuestion{
				{Title: "Thuộc tính nào thường dùng để tạo layout lưới?", Answers: []string{"display: grid", "font-weight", "text-transform", "line-height"}, CorrectAnswer: 0, SelectedAnswer: 0},
				{Title: "Media query dùng để làm gì?", Answers: []string{"Tạo database", "Điều chỉnh giao diện theo điều kiện thiết bị", "Mã hóa mật khẩu", "Gửi email"}, CorrectAnswer: 1, SelectedAnswer: 3},
			},
		}, true
	default:
		return Review{}, false
	}
}

func displayName(account string) string {
	if len(account) >= 2 && (account[0:2] == "sv" || account[0:2] == "SV") {
		return "Sinh viên " + account
	}
	return "Sinh viên demo"
}
