/** One key per intentional action. The same signature reuses it; a new attempt does not. */
export function createIdempotencyKey(randomUUID: () => string = randomKey): string {
  return randomUUID();
}

export function createKeyStore(randomUUID: () => string = randomKey) {
  let current: { signature: string; key: string } | null = null;
  return {
    keyFor(signature: string) {
      if (current?.signature === signature) return current.key;
      const key = randomUUID();
      current = { signature, key };
      return key;
    },
    reset() {
      current = null;
    },
  };
}

function randomKey(): string {
  const bytes = Array.from({ length: 16 }, () => Math.floor(Math.random() * 256));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = bytes.map((b) => b.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
