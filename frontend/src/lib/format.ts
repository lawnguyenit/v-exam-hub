export function formatSeconds(seconds: number) {
  return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

export function formatTime(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString("vi-VN", { hour: "2-digit", minute: "2-digit" });
}

export function statusClass(status: string) {
  if (status === "Đang mở") return "status-open";
  if (status === "Lịch dự kiến") return "status-scheduled";
  return "status-default";
}

export function examTypeClass(examType: string) {
  if (examType === "Thi thử") return "type-practice";
  if (examType === "Chính thức") return "type-official";
  return "type-default";
}
