"use client";
import { useEffect, useRef, type ReactNode } from "react";

export function Modal({
  children,
  className,
  titleID,
  onClose,
}: {
  children: ReactNode;
  className: string;
  titleID: string;
  onClose: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    if (ref.current && !ref.current.open) ref.current.showModal();
  }, []);
  return (
    <dialog
      ref={ref}
      className={className}
      aria-labelledby={titleID}
      onClose={onClose}
      onClick={(event) => {
        if (event.target !== event.currentTarget) return;
        const rect = event.currentTarget.getBoundingClientRect();
        if (
          event.clientX < rect.left ||
          event.clientX > rect.right ||
          event.clientY < rect.top ||
          event.clientY > rect.bottom
        )
          ref.current?.close();
      }}
    >
      <button
        className="close"
        aria-label="Close dialog"
        onClick={() => ref.current?.close()}
      >
        ×
      </button>
      {children}
    </dialog>
  );
}
