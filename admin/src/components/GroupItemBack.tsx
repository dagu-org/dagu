import React from "react";
import { tagColorMapping } from "../consts";

function GroupItemBack() {
  const url = encodeURI("/dags/");
  return (
    <tr>
      <td className="has-text-weight-semibold">
        <a href={url}>../ (upper group)</a>
      </td>
      <td>
        <span
          className="tag has-text-weight-semibold"
          style={tagColorMapping["Group"]}
        >
          Group
        </span>
      </td>
      <td>-</td>
      <td>-</td>
      <td>-</td>
      <td>-</td>
      <td>-</td>
      <td>-</td>
    </tr>
  );
}
export default GroupItemBack;
