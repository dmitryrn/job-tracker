import { type FormEvent, type ReactNode, useEffect, useId, useRef, useState } from "react";
import { rejectJob } from "./api";

type JobRejectionActionProps = {
  jobID: number;
  children: ReactNode;
  className?: string;
  ariaLabel?: string;
  title?: string;
  onRejected: () => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: (message: string) => void;
};

export default function JobRejectionAction({ jobID, children, className, ariaLabel, title, onRejected, onOpen, onClose, onError }: JobRejectionActionProps) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [rejecting, setRejecting] = useState(false);
  const dialogRef = useRef<HTMLDialogElement>(null);
  const reasonID = useId();

  useEffect(() => {
    const dialog = dialogRef.current;
    if (open && dialog && !dialog.open) {
      dialog.showModal();
    }
  }, [open]);

  function openRejection() {
    onOpen?.();
    setReason("");
    setOpen(true);
  }

  async function submitRejection(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedReason = reason.trim();
    if (!trimmedReason || rejecting) {
      return;
    }
    setRejecting(true);
    try {
      await rejectJob(jobID, trimmedReason);
      dialogRef.current?.close();
      onRejected();
    } catch (error) {
      onError?.(error instanceof Error ? error.message : "Could not mark job as won't apply");
      setRejecting(false);
    }
  }

  return (
    <>
      <button type="button" className={className} aria-label={ariaLabel} title={title} onClick={openRejection}>
        {children}
      </button>
      <dialog className="job-rejection-dialog" ref={dialogRef} onClose={() => { setOpen(false); onClose?.(); }}>
        <form onSubmit={(event) => void submitRejection(event)}>
          <header>
            <p className="eyebrow">Won't apply</p>
            <h2>Why are you passing on this role?</h2>
            <p>This removes the role and its match from the Jobs and Matches pages.</p>
          </header>
          <label htmlFor={reasonID}>Reason
            <textarea
              id={reasonID}
              autoFocus
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              onKeyDown={(event) => {
                if (event.ctrlKey && event.key === "Enter") {
                  event.preventDefault();
                  if (reason.trim() && !rejecting) {
                    event.currentTarget.form?.requestSubmit();
                  }
                }
              }}
              placeholder="e.g. Only onsite in Berlin"
              required
              rows={3}
            />
          </label>
          <div className="job-rejection-actions">
            <button type="button" className="secondary-action" disabled={rejecting} onClick={() => dialogRef.current?.close()}>Cancel</button>
            <button type="submit" className="reject-action" disabled={rejecting || !reason.trim()}>{rejecting ? "Saving..." : "Mark as won't apply"}</button>
          </div>
        </form>
      </dialog>
    </>
  );
}
