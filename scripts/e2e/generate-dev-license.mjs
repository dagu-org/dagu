#!/usr/bin/env node

import { generateKeyPairSync, sign } from 'node:crypto';

const nowSeconds = Math.floor(Date.now() / 1000);
const expiresAtSeconds = nowSeconds + 24 * 60 * 60;

const { publicKey, privateKey } = generateKeyPairSync('ed25519');
const publicJwk = publicKey.export({ format: 'jwk' });

if (!publicJwk.x) {
  throw new Error('failed to export Ed25519 public key');
}

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

function base64UrlToBase64(value) {
  const padding = (4 - (value.length % 4)) % 4;
  return value.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat(padding);
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

process.stdout.write(
  JSON.stringify({
    publicKeyB64: Buffer.from(base64UrlToBase64(publicJwk.x), 'base64').toString('base64'),
    token: `${signingInput}.${encodedSignature}`,
  })
);
