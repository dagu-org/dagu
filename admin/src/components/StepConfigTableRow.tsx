import React from "react";
import { Step } from "../models/Step";
import MultilineText from "./MultilineText";

type Props = {
  step: Step;
};

function StepConfigTableRow({ step }: Props) {
  const preconditions = step.Preconditions.map((c) => (
    <li>
      {c.Condition}
      {" => "}
      {c.Expected}
    </li>
  ));
  return (
    <tr>
      <td className="has-text-weight-semibold"> {step.Name} </td>
      <td>
        <MultilineText>{step.Description}</MultilineText>
      </td>
      <td> {step.Command} </td>
      <td> {step.Args ? step.Args.join(" ") : ""} </td>
      <td> {step.Dir} </td>
      <td> {step.RepeatPolicy.Repeat ? "Repeat" : "-"} </td>
      <td>
        <ul> {preconditions} </ul>
      </td>
    </tr>
  );
}

export default StepConfigTableRow;
