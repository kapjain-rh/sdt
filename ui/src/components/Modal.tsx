"use client";

import { useEffect, useRef } from "react";
import { X } from "lucide-react";

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  wide?: boolean;
  children: React.ReactNode;
  footer?: React.ReactNode;
}

export function Modal({ open, onClose, title, wide, children, footer }: ModalProps) {
  const overlayRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 bg-black/40 flex items-start justify-center pt-16 px-4"
      onClick={(e) => {
        if (e.target === overlayRef.current) onClose();
      }}
    >
      <div
        className={`bg-white rounded-lg shadow-2xl flex flex-col max-h-[85vh] ${
          wide ? "w-[900px] max-w-[95vw]" : "w-[640px] max-w-[90vw]"
        }`}
      >
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-slate-900">{title}</h2>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-slate-100 text-slate-400 hover:text-slate-600 transition-colors"
          >
            <X size={20} />
          </button>
        </div>
        <div className="px-6 py-5 overflow-y-auto flex-1">{children}</div>
        {footer && (
          <div className="flex justify-end gap-2 px-6 py-4 border-t border-border bg-slate-50 rounded-b-lg">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
