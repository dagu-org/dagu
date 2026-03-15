const base64ChunkSize = 0x8000;

export function base64ToBytes(value: string): Uint8Array {
  const binary = atob(value);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (let i = 0; i < bytes.length; i += base64ChunkSize) {
    const chunk = bytes.subarray(i, i + base64ChunkSize);
    binary += String.fromCharCode(...chunk);
  }
  return btoa(binary);
}

export function stringToBase64(value: string): string {
  return bytesToBase64(new TextEncoder().encode(value));
}

export function binaryStringToBase64(value: string): string {
  const bytes = new Uint8Array(value.length);
  for (let i = 0; i < value.length; i++) {
    bytes[i] = value.charCodeAt(i) & 0xff;
  }
  return bytesToBase64(bytes);
}
