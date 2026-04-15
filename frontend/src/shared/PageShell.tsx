import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { Brand } from "./Brand";

export function PageShell({ backTo, children }: { backTo: string; children: ReactNode }) {
  return (
    <div className="app-shell">
      <header className="app-header">
        <Brand to={backTo} />
        <Link className="ghost-btn" to={backTo}>Về dashboard</Link>
      </header>
      <div className="app-content">{children}</div>
      <footer className="app-footer">
        <span>Email: lawnguyenit@gmail.com</span>
        <span>SĐT: 0916203547</span>
        <span>Design by Lâm Nguyên</span>
      </footer>
    </div>
  );
}
