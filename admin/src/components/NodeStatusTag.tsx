import React from "react";
import { nodeStatusColorMapping } from "../consts";
import { NodeStatus } from "../models/Node";

type Props = {
  status: NodeStatus;
  children: React.ReactNode;
};

function NodeStatusTag({ status, children }: Props) {
  const style = React.useMemo(() => {
    return nodeStatusColorMapping[status] || {};
  }, [status]);
  return (
    <span className="tag has-text-weight-semibold" style={style}>
      {children}
    </span>
  );
}

export default NodeStatusTag;
