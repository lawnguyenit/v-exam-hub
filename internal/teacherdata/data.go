package teacherdata

type Dashboard struct {
	Profile Profile       `json:"profile"`
	Summary Summary       `json:"summary"`
	Exams   []ExamSummary `json:"exams"`
}

type Profile struct {
	DisplayName string `json:"displayName"`
	Department  string `json:"department"`
}

type Summary struct {
	ExamCount       int `json:"examCount"`
	OpenExamCount   int `json:"openExamCount"`
	StudentCount    int `json:"studentCount"`
	NeedReviewCount int `json:"needReviewCount"`
}

type ExamSummary struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	TargetClass string  `json:"targetClass"`
	StartTime   string  `json:"startTime"`
	Average     float64 `json:"average"`
	Submitted   int     `json:"submitted"`
	Total       int     `json:"total"`
}

type ExamDetail struct {
	ID          string                     `json:"id"`
	Title       string                     `json:"title"`
	Status      string                     `json:"status"`
	TargetClass string                     `json:"targetClass"`
	StartTime   string                     `json:"startTime"`
	Metrics     []Metric                   `json:"metrics"`
	Tables      map[string]StatisticsTable `json:"tables"`
	Students    []StudentAttemptDetail     `json:"students"`
}

type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type StatisticsTable struct {
	Title   string     `json:"title"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

type StudentAttemptDetail struct {
	Name       string      `json:"name"`
	Progress   string      `json:"progress"`
	Warning    string      `json:"warning"`
	Score      string      `json:"score"`
	Duration   string      `json:"duration"`
	WrongItems []WrongItem `json:"wrongItems"`
}

type WrongItem struct {
	Question string `json:"question"`
	Selected string `json:"selected"`
	Correct  string `json:"correct"`
	Note     string `json:"note"`
}

func DashboardFor(account string) Dashboard {
	if account == "" {
		account = "GV-CNTT-01"
	}

	exams := examSummaries()
	return Dashboard{
		Profile: Profile{
			DisplayName: "Giáo viên " + account,
			Department:  "Khoa Công nghệ thông tin",
		},
		Summary: Summary{
			ExamCount:       len(exams),
			OpenExamCount:   2,
			StudentCount:    126,
			NeedReviewCount: 4,
		},
		Exams: exams,
	}
}

func ExamDetailByID(id string) (ExamDetail, bool) {
	switch id {
	case "go-basics-demo":
		return ExamDetail{
			ID:          "go-basics-demo",
			Title:       "Lập trình cơ sở với Go",
			Status:      "Đang mở",
			TargetClass: "CNTT K48",
			StartTime:   "Hôm nay 08:00",
			Metrics: []Metric{
				{Label: "Đã nộp", Value: "41/52"},
				{Label: "Điểm cao nhất", Value: "9.8"},
				{Label: "Điểm trung bình", Value: "7.4"},
				{Label: "Thời gian TB", Value: "32 phút"},
			},
			Tables: map[string]StatisticsTable{
				"top_students": {
					Title:   "Sinh viên làm tốt nhất",
					Columns: []string{"Sinh viên", "Điểm", "Thời gian", "Trạng thái"},
					Rows: [][]string{
						{"Nguyễn An", "9.8", "27 phút", "Đã nộp"},
						{"Lê Minh Thu", "9.4", "31 phút", "Đã nộp"},
						{"Trần Bảo Lâm", "9.0", "35 phút", "Đã nộp"},
					},
				},
				"score_distribution": {
					Title:   "Phân bố điểm",
					Columns: []string{"Khoảng điểm", "Số sinh viên", "Tỷ lệ"},
					Rows: [][]string{
						{"9 - 10", "6", "14.6%"},
						{"7 - 8.9", "22", "53.7%"},
						{"5 - 6.9", "10", "24.4%"},
						{"Dưới 5", "3", "7.3%"},
					},
				},
				"question_difficulty": {
					Title:   "Câu dễ sai nhất",
					Columns: []string{"Câu", "Tỷ lệ sai", "Ghi chú"},
					Rows: [][]string{
						{"#5", "48%", "Channel và goroutine"},
						{"#8", "41%", "Error handling"},
						{"#11", "37%", "Defer"},
					},
				},
				"live_status": {
					Title:   "Trạng thái phòng thi",
					Columns: []string{"Sinh viên", "Tiến trình", "Cảnh báo"},
					Rows:    [][]string{},
				},
			},
			Students: liveStudents(),
		}, true
	case "database-schedule":
		return ExamDetail{
			ID:          "database-schedule",
			Title:       "Cơ sở dữ liệu",
			Status:      "Lịch dự kiến",
			TargetClass: "CNTT K48",
			StartTime:   "15/04 - 08:00",
			Metrics: []Metric{
				{Label: "Số câu", Value: "30"},
				{Label: "Thời lượng", Value: "45 phút"},
				{Label: "Lớp áp dụng", Value: "CNTT K48"},
				{Label: "Trạng thái", Value: "Chờ mở"},
			},
			Tables: map[string]StatisticsTable{
				"top_students": {
					Title:   "Chưa có dữ liệu làm bài",
					Columns: []string{"Mục", "Giá trị"},
					Rows:    [][]string{{"Ghi chú", "Bảng thống kê sẽ xuất hiện khi sinh viên bắt đầu làm bài."}},
				},
				"score_distribution": {
					Title:   "Chưa có phân bố điểm",
					Columns: []string{"Mục", "Giá trị"},
					Rows:    [][]string{{"Trạng thái", "Đang chờ đến giờ mở bài."}},
				},
			},
		}, true
	default:
		return ExamDetail{}, false
	}
}

func examSummaries() []ExamSummary {
	return []ExamSummary{
		{ID: "go-basics-demo", Title: "Lập trình cơ sở với Go", Status: "Đang mở", TargetClass: "CNTT K48", StartTime: "Hôm nay 08:00", Average: 7.4, Submitted: 41, Total: 52},
		{ID: "database-schedule", Title: "Cơ sở dữ liệu", Status: "Lịch dự kiến", TargetClass: "CNTT K48", StartTime: "15/04 - 08:00", Average: 0, Submitted: 0, Total: 52},
		{ID: "network-practice", Title: "Kiến thức mạng máy tính", Status: "Thi thử", TargetClass: "CNTT K49", StartTime: "Đã mở", Average: 6.9, Submitted: 31, Total: 44},
	}
}

func liveStudents() []StudentAttemptDetail {
	return []StudentAttemptDetail{
		{Name: "Phạm Gia Huy", Progress: "8/12", Warning: "Ổn định", Score: "7.2", Duration: "29 phút", WrongItems: []WrongItem{{Question: "Channel dùng để làm gì?", Selected: "Vẽ biểu đồ", Correct: "Trao đổi dữ liệu giữa goroutine", Note: "Nhầm giữa phần UI và cơ chế đồng thời."}}},
		{Name: "Đỗ Hải Nam", Progress: "4/12", Warning: "Rời tab 1 lần", Score: "Đang làm", Duration: "18 phút", WrongItems: []WrongItem{{Question: "Goroutine dùng để làm gì?", Selected: "Tạo database", Correct: "Chạy tác vụ đồng thời", Note: "Cần xem lại concurrency cơ bản."}}},
		{Name: "Ngô Khánh Linh", Progress: "12/12", Warning: "Đã nộp", Score: "8.8", Duration: "34 phút", WrongItems: []WrongItem{{Question: "defer chạy khi nào?", Selected: "Khi khai báo", Correct: "Khi hàm bao quanh kết thúc", Note: "Sai thời điểm thực thi defer."}}},
		{Name: "Nguyễn An", Progress: "9/12", Warning: "Ổn định", Score: "Đang làm", Duration: "25 phút", WrongItems: []WrongItem{{Question: "go run . dùng để làm gì?", Selected: "Publish package", Correct: "Chạy ứng dụng Go cục bộ", Note: "Nhầm thao tác chạy và publish."}}},
		{Name: "Lê Minh Thu", Progress: "12/12", Warning: "Đã nộp", Score: "9.4", Duration: "31 phút", WrongItems: []WrongItem{{Question: "Kiểu lỗi trong Go thường xử lý thế nào?", Selected: "try/catch", Correct: "Trả về error và kiểm tra", Note: "Nhầm với ngôn ngữ khác."}}},
		{Name: "Trần Bảo Lâm", Progress: "7/12", Warning: "Mất kết nối 1 lần", Score: "Đang làm", Duration: "22 phút", WrongItems: []WrongItem{{Question: "Channel nhận dữ liệu bằng toán tử nào?", Selected: "=>", Correct: "<-", Note: "Sai cú pháp channel."}}},
		{Name: "Vũ Hoàng Long", Progress: "5/12", Warning: "Ổn định", Score: "Đang làm", Duration: "16 phút", WrongItems: []WrongItem{{Question: "package main dùng cho trường hợp nào?", Selected: "Thư viện dùng chung", Correct: "Chương trình chạy được", Note: "Cần phân biệt binary và library."}}},
		{Name: "Mai Phương Anh", Progress: "10/12", Warning: "Ổn định", Score: "Đang làm", Duration: "28 phút", WrongItems: []WrongItem{{Question: "interface{} biểu thị gì?", Selected: "Chỉ string", Correct: "Có thể giữ nhiều kiểu", Note: "Cần ôn lại interface."}}},
		{Name: "Hoàng Minh Quân", Progress: "3/12", Warning: "Rời tab 2 lần", Score: "Đang làm", Duration: "12 phút", WrongItems: []WrongItem{{Question: "HTTP 404 là gì?", Selected: "Thành công", Correct: "Không tìm thấy", Note: "Cần xem lại status code."}}},
		{Name: "Bùi Khánh Vy", Progress: "11/12", Warning: "Ổn định", Score: "Đang làm", Duration: "33 phút", WrongItems: []WrongItem{{Question: "slice khác array ở điểm nào?", Selected: "Không có length", Correct: "Slice là view động trên array", Note: "Cần xem lại slice."}}},
	}
}
