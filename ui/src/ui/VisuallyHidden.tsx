import styled from '@emotion/styled';

/** A component which hides its children. The children will still be available for screen readers.
 * @see {@link https://www.a11yproject.com/posts/how-to-hide-content/} for further information.
 */
const VisuallyHidden = styled.span`
  clip: rect(0 0 0 0);
  clip-path: inset(50%);
  height: 1px;
  overflow: hidden;
  position: absolute;
  white-space: nowrap;
  width: 1px;
`;

export default VisuallyHidden;
