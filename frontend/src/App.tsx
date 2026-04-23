import { Navigate, Route, Routes } from "react-router-dom";
import { AdminTeachers } from "./pages/admin/AdminTeachers";
import { LoginPage } from "./pages/auth/LoginPage";
import { RoleSelect } from "./pages/auth/RoleSelect";
import { StudentDashboard } from "./pages/student/StudentDashboard";
import { StudentExam } from "./pages/student/StudentExam";
import { StudentReview } from "./pages/student/StudentReview";
import { TeacherCreateExam } from "./pages/teacher/TeacherCreateExam";
import { TeacherDashboard } from "./pages/teacher/TeacherDashboard";
import { TeacherStudents } from "./pages/teacher/TeacherStudents";

function App() {
  return (
    <Routes>
      <Route path="/" element={<RoleSelect />} />
      <Route path="/login" element={<Navigate to="/" replace />} />
      <Route path="/login/student" element={<LoginPage role="student" />} />
      <Route path="/login/teacher" element={<LoginPage role="teacher" />} />
      <Route path="/login/admin" element={<LoginPage role="admin" />} />
      <Route path="/admin" element={<AdminTeachers />} />
      <Route path="/student" element={<StudentDashboard />} />
      <Route path="/student/exam" element={<StudentExam />} />
      <Route path="/student/review" element={<StudentReview />} />
      <Route path="/teacher" element={<TeacherDashboard />} />
      <Route path="/teacher/create" element={<TeacherCreateExam />} />
      <Route path="/teacher/students" element={<TeacherStudents />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default App;
