#!/usr/bin/env node

import { createPrivateKey, sign } from 'node:crypto';

const privKeyB64 = process.env.DAGU_LICENSE_PRIVKEY_B64;
if (!privKeyB64) {
  process.stderr.write('DAGU_LICENSE_PRIVKEY_B64 not set; skipping license generation\n');
  process.exit(0);
}

// Go ed25519.PrivateKey is seed (32 bytes) + public key (32 bytes).
const rawKey = Buffer.from(privKeyB64, 'base64');
if (rawKey.length !== 64) {
  process.stderr.write(
    `DAGU_LICENSE_PRIVKEY_B64 must decode to 64 bytes (got ${rawKey.length})\n`
  );
  process.exit(1);
}
const seed = rawKey.subarray(0, 32);
const pub = rawKey.subarray(32);
const privateKey = createPrivateKey({
  key: { kty: 'OKP', crv: 'Ed25519', d: seed.toString('base64url'), x: pub.toString('base64url') },
  format: 'jwk',
});

const nowSeconds = Math.floor(Date.now() / 1000);
const expiresAtSeconds = nowSeconds + 24 * 60 * 60;

const claims = {
  iss: 'dagu-e2e',
  sub: 'dagu-e2e-license',
  iat: nowSeconds,
  exp: expiresAtSeconds,
  cv: 1,
  plan: 'pro',
  features: ['audit', 'rbac', 'sso'],
  activation_id: 'dagu-e2e-license',
};

const header = {
  alg: 'EdDSA',
  typ: 'JWT',
};

function base64UrlEncode(value) {
  return Buffer.from(value)
    .toString('base64')
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/g, '');
}

const encodedHeader = base64UrlEncode(JSON.stringify(header));
const encodedClaims = base64UrlEncode(JSON.stringify(claims));
const signingInput = `${encodedHeader}.${encodedClaims}`;
const signature = sign(null, Buffer.from(signingInput), privateKey);
const encodedSignature = signature
  .toString('base64')
  .replace(/\+/g, '-')
  .replace(/\//g, '_')
  .replace(/=+$/g, '');

process.stdout.write(`${signingInput}.${encodedSignature}`);
