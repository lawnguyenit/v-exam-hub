package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"website-exam/internal/accountdata"
	"website-exam/internal/authsession"
	"website-exam/internal/httpapi"
	"website-exam/internal/importdata"
	"website-exam/internal/storage"
	"website-exam/internal/teacherdata"

	"github.com/jackc/pgx/v5/pgxpool"
)

func handleTeacherProfileUpdate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload teacherdata.ProfileUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c thÃ´ng tin há»“ sÆ¡ giÃ¡o viÃªn", http.StatusBadRequest)
			return
		}
		payload.Username = auth.Username
		profile, err := teacherdata.UpdateProfile(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng lÆ°u Ä‘Æ°á»£c há»“ sÆ¡ giÃ¡o viÃªn: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, profile)
	}
}

func handleTeacherQuestionBank(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items, err := teacherdata.QuestionBank(r.Context(), db, auth.Username)
		if err != nil {
			http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c ngÃ¢n hÃ ng cÃ¢u há»i: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, items)
	}
}

func handleTeacherQuestionBankSource(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teacher/question-bank/"), "/")
		batchID, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || batchID <= 0 {
			http.NotFound(w, r)
			return
		}
		result, err := teacherdata.DeleteQuestionBankSource(r.Context(), db, batchID, auth.Username)
		if err != nil {
			http.Error(w, "Khong xoa duoc bo de cuong: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, result)
	}
}

func handleTeacherExamCreate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload teacherdata.ExamCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c cáº¥u hÃ¬nh bÃ i kiá»ƒm tra", http.StatusBadRequest)
			return
		}
		payload.CreatedBy = auth.Username
		result, err := teacherdata.CreateExam(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng táº¡o Ä‘Æ°á»£c bÃ i kiá»ƒm tra: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, result)
	}
}

func handleTeacherImportItemCreate(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64                     `json:"importBatchId"`
		Question      importdata.ParsedQuestion `json:"question"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Khong doc duoc du lieu cau hoi", http.StatusBadRequest)
			return
		}
		if !authsession.TeacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		question, err := importdata.CreateImportItem(r.Context(), db, payload.ImportBatchID, payload.Question)
		if err != nil {
			http.Error(w, "Khong tao duoc cau hoi: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, question)
	}
}

func handleTeacherImportItemSave(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64                     `json:"importBatchId"`
		Question      importdata.ParsedQuestion `json:"question"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c dá»¯ liá»‡u cÃ¢u há»i", http.StatusBadRequest)
			return
		}
		if !authsession.TeacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		if err := importdata.UpdateImportItem(r.Context(), db, payload.ImportBatchID, payload.Question); err != nil {
			http.Error(w, "KhÃ´ng lÆ°u Ä‘Æ°á»£c cÃ¢u há»i: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, map[string]any{"ok": true})
	}
}

func handleTeacherImportItemDelete(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64 `json:"importBatchId"`
		ImportItemID  int64 `json:"importItemId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c cÃ¢u cáº§n xoÃ¡", http.StatusBadRequest)
			return
		}
		if !authsession.TeacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		if err := importdata.RejectImportItem(r.Context(), db, payload.ImportBatchID, payload.ImportItemID); err != nil {
			http.Error(w, "KhÃ´ng xoÃ¡ Ä‘Æ°á»£c cÃ¢u khá»i batch: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, map[string]any{"ok": true})
	}
}

func handleTeacherImportApprovePass(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64 `json:"importBatchId"`
		TargetBatchID int64 `json:"targetBatchId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c batch import", http.StatusBadRequest)
			return
		}
		if !authsession.TeacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) || (payload.TargetBatchID > 0 && !authsession.TeacherOwnsBatch(r.Context(), db, payload.TargetBatchID, auth.UserID)) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		result, err := importdata.ApprovePassedImportItemsToSource(r.Context(), db, payload.ImportBatchID, payload.TargetBatchID)
		if err != nil {
			http.Error(w, "KhÃ´ng lÆ°u Ä‘Æ°á»£c cÃ¢u pass: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, result)
	}
}

func handleTeacherClasses(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		classes, err := accountdata.ListClasses(r.Context(), db, auth.UserID)
		if err != nil {
			http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c danh sÃ¡ch lá»›p: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, classes)
	}
}

func handleTeacherClassDetail(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teacher/classes/"), "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || parts[0] == "" || parts[0] == "import-students" {
			http.NotFound(w, r)
			return
		}
		classID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || classID <= 0 {
			http.Error(w, "mÃ£ lá»›p khÃ´ng há»£p lá»‡", http.StatusBadRequest)
			return
		}
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				detail, err := accountdata.ClassDetailByID(r.Context(), db, auth.UserID, classID)
				if err != nil {
					http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c chi tiáº¿t lá»›p: "+err.Error(), http.StatusBadRequest)
					return
				}
				httpapi.WriteJSON(w, detail)
			case http.MethodPatch:
				var payload accountdata.ClassUpdateRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c dá»¯ liá»‡u lá»›p", http.StatusBadRequest)
					return
				}
				updated, err := accountdata.UpdateClass(r.Context(), db, auth.UserID, classID, payload)
				if err != nil {
					http.Error(w, "KhÃ´ng sá»­a Ä‘Æ°á»£c lá»›p: "+err.Error(), http.StatusBadRequest)
					return
				}
				httpapi.WriteJSON(w, updated)
			case http.MethodDelete:
				if err := accountdata.ArchiveClass(r.Context(), db, auth.UserID, classID); err != nil {
					http.Error(w, "KhÃ´ng xÃ³a Ä‘Æ°á»£c lá»›p: "+err.Error(), http.StatusBadRequest)
					return
				}
				httpapi.WriteJSON(w, map[string]any{"ok": true})
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if len(parts) == 3 && parts[1] == "members" && r.Method == http.MethodDelete {
			studentUserID, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil || studentUserID <= 0 {
				http.Error(w, "mÃ£ sinh viÃªn khÃ´ng há»£p lá»‡", http.StatusBadRequest)
				return
			}
			if err := accountdata.RemoveClassMember(r.Context(), db, auth.UserID, classID, studentUserID); err != nil {
				http.Error(w, "KhÃ´ng xÃ³a Ä‘Æ°á»£c sinh viÃªn khá»i lá»›p: "+err.Error(), http.StatusBadRequest)
				return
			}
			httpapi.WriteJSON(w, map[string]any{"ok": true})
			return
		}
		http.NotFound(w, r)
	}
}

func handleTeacherClassStudentImport(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c file danh sÃ¡ch sinh viÃªn", http.StatusBadRequest)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "Thiáº¿u file danh sÃ¡ch sinh viÃªn", http.StatusBadRequest)
				return
			}
			defer file.Close()
			content, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c ná»™i dung file", http.StatusBadRequest)
				return
			}
			result, err := accountdata.ImportStudentsFromXLSX(
				r.Context(),
				db,
				auth.UserID,
				r.FormValue("classCode"),
				r.FormValue("className"),
				header.Filename,
				content,
			)
			if err != nil {
				http.Error(w, "KhÃ´ng táº¡o Ä‘Æ°á»£c tÃ i khoáº£n sinh viÃªn: "+err.Error(), http.StatusBadRequest)
				return
			}
			httpapi.WriteJSON(w, result)
			return
		}
		var payload accountdata.StudentImportRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c danh sÃ¡ch sinh viÃªn", http.StatusBadRequest)
			return
		}
		result, err := accountdata.ImportStudents(r.Context(), db, auth.UserID, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng táº¡o Ä‘Æ°á»£c tÃ i khoáº£n sinh viÃªn: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, result)
	}
}

func handleTeacherStudentPasswordUpdate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authsession.Require(r.Context(), db, w, r, "teacher"); !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload accountdata.StudentPasswordUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c dá»¯ liá»‡u Ä‘á»•i máº­t kháº©u", http.StatusBadRequest)
			return
		}
		if err := accountdata.UpdateStudentPassword(r.Context(), db, payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»•i Ä‘Æ°á»£c máº­t kháº©u: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, map[string]any{"ok": true})
	}
}

func handleTeacherImportAsset(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authsession.Require(r.Context(), db, w, r); !ok {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/teacher/import/assets/")
		parts := strings.Split(strings.Trim(trimmed, "/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		batchID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || batchID <= 0 {
			http.NotFound(w, r)
			return
		}
		displayOrder, err := strconv.Atoi(parts[1])
		if err != nil || displayOrder <= 0 {
			http.NotFound(w, r)
			return
		}

		var fileURL, mimeType string
		err = db.QueryRow(r.Context(), `
			SELECT file_url, COALESCE(mime_type, 'application/octet-stream')
			FROM import_item_assets
			WHERE batch_id = $1 AND display_order = $2
			ORDER BY id
			LIMIT 1
		`, batchID, displayOrder).Scan(&fileURL, &mimeType)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		absPath, ok := storage.ResolveImportFile(fileURL)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeFile(w, r, absPath)
	}
}

func handleTeacherImportParse(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := authsession.Require(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c file upload", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Thiáº¿u file Ä‘á» thi", http.StatusBadRequest)
			return
		}

		result, err := importdata.ParseUpload(file, header)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := importdata.SaveImport(r.Context(), db, &result, auth.Username); err != nil {
			http.Error(w, "KhÃ´ng lÆ°u Ä‘Æ°á»£c import vÃ o database: "+err.Error(), http.StatusInternalServerError)
			return
		}
		httpapi.WriteJSON(w, result)
	}
}
