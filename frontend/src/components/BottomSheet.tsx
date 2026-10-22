import { useTranslation } from "react-i18next";
import { useEffect, useRef, type ReactNode } from "react";

interface Props {
  id: string;
  title: string;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
}

export function BottomSheet({ id, title, onClose, children, footer }: Props) {
  const { t } = useTranslation();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const backdropPress = useRef(false);

  useEffect(() => {
    const dialog = dialogRef.current!;
    const previousFocus = document.activeElement;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    dialog.showModal();
    return () => {
      dialog.close();
      document.body.style.overflow = previousOverflow;
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected)
        previousFocus.focus({ preventScroll: true });
    };
  }, []);

  return (
    <dialog
      ref={dialogRef}
      id={id}
      className="bottom-sheet"
      aria-labelledby={`${id}-title`}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onKeyDown={(event) => {
        if (event.key !== "Tab") return;
        const controls = Array.from(
          event.currentTarget.querySelectorAll<HTMLElement>(
            "button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]",
          ),
        ).filter(
          (element) =>
            element.tabIndex >= 0 && element.getClientRects().length > 0,
        );
        const first = controls[0];
        const last = controls.at(-1);
        if (event.shiftKey && document.activeElement === first && last) {
          event.preventDefault();
          last.focus();
        } else if (
          !event.shiftKey &&
          document.activeElement === last &&
          first
        ) {
          event.preventDefault();
          first.focus();
        }
      }}
      onPointerDown={(event) => {
        backdropPress.current = event.target === event.currentTarget;
      }}
      onClick={(event) => {
        if (backdropPress.current && event.target === event.currentTarget)
          onClose();
        backdropPress.current = false;
      }}
    >
      <div className="flex max-h-[85dvh] flex-col">
        <div
          className="mx-auto mt-3 h-1 w-10 shrink-0 rounded-full bg-line"
          aria-hidden="true"
        />
        <header className="flex shrink-0 items-center justify-between gap-4 border-b border-line px-5 py-5 sm:px-8">
          <h2 id={`${id}-title`} className="font-display text-3xl">
            {title}
          </h2>
          <button
            type="button"
            aria-label={t("common.close")}
            className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border border-line text-xl hover:border-ink"
            onClick={onClose}
          >
            <span aria-hidden="true">×</span>
          </button>
        </header>
        <div className="min-h-0 overflow-y-auto overscroll-contain px-5 py-6 sm:px-8">
          {children}
        </div>
        {footer && (
          <footer className="bottom-sheet-footer shrink-0 border-t border-line px-5 pt-4 sm:px-8">
            {footer}
          </footer>
        )}
      </div>
    </dialog>
  );
}
