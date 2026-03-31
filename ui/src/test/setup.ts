import '@testing-library/jest-dom/vitest';

// jsdom doesn't implement scrollIntoView
Element.prototype.scrollIntoView = () => {};

// Radix Select uses pointer capture APIs that jsdom does not provide.
if (!Element.prototype.hasPointerCapture) {
  Element.prototype.hasPointerCapture = () => false;
}

if (!Element.prototype.setPointerCapture) {
  Element.prototype.setPointerCapture = () => {};
}

if (!Element.prototype.releasePointerCapture) {
  Element.prototype.releasePointerCapture = () => {};
}
