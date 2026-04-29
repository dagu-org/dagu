import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { dereferenceSchema, type JSONSchema } from '@/lib/schema-utils';

describe('dereferenceSchema', () => {
  it('handles recursive internal references without overflowing the stack', () => {
    const schema: JSONSchema = {
      type: 'object',
      properties: {
        root: {
          $ref: '#/definitions/node',
        },
      },
      definitions: {
        node: {
          type: 'object',
          properties: {
            name: { type: 'string' },
            next: {
              $ref: '#/definitions/node',
            },
          },
        },
      },
    };

    expect(() => dereferenceSchema(schema)).not.toThrow();
    const dereferenced = dereferenceSchema(schema);
    expect(dereferenced.properties?.root?.properties?.next).toBeDefined();
  });

  it('dereferences the bundled DAG schema used by the editor', () => {
    const schemaPath = path.resolve(
      path.dirname(fileURLToPath(import.meta.url)),
      '../../../../schemas/dag.schema.json'
    );
    const rawSchema = JSON.parse(
      fs.readFileSync(schemaPath, 'utf8')
    ) as JSONSchema;

    expect(() => dereferenceSchema(rawSchema)).not.toThrow();
    const dereferenced = dereferenceSchema(rawSchema);
    expect(Object.keys(dereferenced.properties ?? {})).toContain('name');
  });
});
