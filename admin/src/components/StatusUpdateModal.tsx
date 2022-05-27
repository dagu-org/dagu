import React from "react";
import { Step } from "../models/Step";

type Props = {
  visible: boolean;
  dismissModal: () => void;
  step?: Step;
  onSubmit: (step: Step, action: string) => void;
};

function StatusUpdateModal({ visible, dismissModal, step, onSubmit }: Props) {
  const modalStyle = {
    display: visible ? "flex" : "none",
  };
  React.useEffect(() => {
    const callback = (event: KeyboardEvent) => {
      const e = event || window.event;
      if (e.key == "Escape" || e.key == "Esc") {
        dismissModal();
      }
    };
    document.addEventListener("keydown", callback);
    return () => {
      document.removeEventListener("keydown", callback);
    };
  }, [dismissModal]);
  if (!step) {
    return null;
  }
  return (
    <div className="modal is-active" style={modalStyle}>
      <div className="modal-background"></div>
      <div className="modal-card">
        <header className="modal-card-head">
          <p className="modal-card-title">Update status of "{step.Name}"</p>
          <button
            className="delete"
            aria-label="close"
            onClick={dismissModal}
          ></button>
        </header>
        <section className="modal-card-body">
          <div className="mr-4 pt-4 is-flex is-flex-direction-row">
            <button
              className="button is-info"
              onClick={() => onSubmit(step, "mark-success")}
            >
              <span>Mark Success</span>
            </button>
            <button
              className="button is-info ml-4"
              onClick={() => onSubmit(step, "mark-failed")}
            >
              <span>Mark Failed</span>
            </button>
          </div>
        </section>
        <footer className="modal-card-foot">
          <button className="button" onClick={dismissModal}>
            Cancel
          </button>
        </footer>
      </div>
    </div>
  );
}

export default StatusUpdateModal;
