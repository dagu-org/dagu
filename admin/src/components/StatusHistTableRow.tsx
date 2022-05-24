import React, { CSSProperties } from "react";
import { DagStatus } from "../api/Workflow";
import { statusColorMapping } from "../consts";

type Props = {
  data: DagStatus;
  onSelect: (idx: number) => void;
  idx: number;
};

function StatusHistTableRow({ data, onSelect, idx }: Props) {
  const vals = React.useMemo(() => {
    return data.Vals.reverse();
  }, [data]);
  return (
    <tr>
      <td className="has-text-weight-semibold">{data.Name}</td>
      {vals.map((status, i) => {
        const style: CSSProperties = { ...circleStyle };
        const tdStyle: CSSProperties = {};
        if (i == idx) {
          tdStyle.backgroundColor = "#FFDDAD";
        }
        if (status != 0) {
          style.backgroundColor = statusColorMapping[status].backgroundColor;
          style.color = statusColorMapping[status].color;
        }
        return (
          <td
            key={i}
            onClick={() => {
              onSelect(i);
            }}
            style={tdStyle}
          >
            {status != 0 ? <div style={style}></div> : ""}
          </td>
        );
      })}
    </tr>
  );
}

export default StatusHistTableRow;

const circleStyle = {
  width: "20px",
  height: "20px",
  borderRadius: "50%",
  backgroundColor: "#000000",
};
