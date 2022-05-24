import React from "react";
import { tagColorMapping } from "../consts";
import { Group } from "../models/Group";

type Props = {
  group: Group;
};

function GroupItem({ group }: Props) {
  const url = encodeURI("/dags/?group=" + group.Name);
  return (
    <tr>
      <td className="has-text-weight-semibold">
        <a href={url}>{group.Name}</a>
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
export default GroupItem;
