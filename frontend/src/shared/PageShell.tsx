import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { Brand } from "./Brand";

export function PageShell({ backTo, children }: { backTo: string; children: ReactNode }) {
  return (
    <>
      <header className="app-header">
        <Brand to={backTo} />
        <Link className="ghost-btn" to={backTo}>Về dashboard</Link>
      </header>
      {children}
    </>
  );
}
