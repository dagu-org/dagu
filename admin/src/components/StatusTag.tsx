import React from "react";
import { statusColorMapping } from "../consts";
import { SchedulerStatus } from "../models/Status";

type Props = {
  status: SchedulerStatus;
  children: React.ReactNode;
}

  function StatusTag({ status, children }: Props) {
    const style = React.useMemo(() => {
      return statusColorMapping[status] || {};
    }, [status]);
    return (
      <span className="tag has-text-weight-semibold" style={style}>
        {children}
      </span>
    );
  }

export default StatusTag;