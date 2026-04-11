import { describe, expect, it } from 'vitest';
import {
  buildAugmentedDAGSchema,
  extractLocalCustomStepTypeHints,
  mergeCustomStepTypeHints,
} from '../customStepSchema';
import { getSchemaAtPath, type JSONSchema } from '@/lib/schema-utils';

const baseSchema: JSONSchema = {
  type: 'object',
  properties: {
    steps: {
      type: 'array',
      items: {
        $ref: '#/definitions/step',
      },
    },
  },
  definitions: {
    executorType: {
      anyOf: [
        {
          type: 'string',
          enum: ['command'],
        },
        {
          type: 'string',
          pattern: '^[A-Za-z][A-Za-z0-9_-]*$',
        },
      ],
    },
    step: {
      type: 'object',
      properties: {
        type: {
          $ref: '#/definitions/executorType',
        },
        config: {
          type: 'object',
        },
      },
      allOf: [],
    },
  },
};

describe('customStepSchema', () => {
  it('extracts local custom step types from YAML', () => {
    const result = extractLocalCustomStepTypeHints(`
step_types:
  greet:
    type: command
    description: Send a greeting
    input_schema:
      type: object
      additionalProperties: false
      required: [message]
      properties:
        message:
          type: string
    template:
      exec:
        command: echo
        args:
          - {$input: message}
`);

    expect(result.ok).toBe(true);
    expect(result.stepTypes).toHaveLength(1);
    expect(result.stepTypes[0]).toMatchObject({
      name: 'greet',
      targetType: 'command',
      description: 'Send a greeting',
    });
  });

  it('preserves the local definition when it overrides an inherited name', () => {
    const merged = mergeCustomStepTypeHints(
      [
        {
          name: 'greet',
          targetType: 'command',
          inputSchema: {
            type: 'object',
            properties: {
              message: { type: 'string' },
            },
          },
        },
      ],
      [
        {
          name: 'greet',
          targetType: 'command',
          inputSchema: {
            type: 'object',
            properties: {
              count: { type: 'integer' },
            },
          },
        },
      ]
    );

    const schema = buildAugmentedDAGSchema(baseSchema, merged);
    const propertySchema = getSchemaAtPath(
      schema,
      ['steps', '0', 'config', 'count'],
      `
steps:
  - type: greet
    config:
      count: 1
`
    );

    expect(propertySchema).toMatchObject({ type: 'integer' });
  });

  it('resolves internal refs inside local custom input schemas', () => {
    const schema = buildAugmentedDAGSchema(baseSchema, [
      {
        name: 'greet',
        targetType: 'command',
        inputSchema: {
          type: 'object',
          properties: {
            profile: {
              $ref: '#/definitions/profile',
            },
          },
          definitions: {
            profile: {
              type: 'object',
              properties: {
                message: {
                  type: 'string',
                },
              },
            },
          },
        },
      },
    ]);

    const propertySchema = getSchemaAtPath(
      schema,
      ['steps', '0', 'config', 'profile', 'message'],
      `
steps:
  - type: greet
    config:
      profile:
        message: hello
`
    );

    expect(propertySchema).toMatchObject({ type: 'string' });
  });

  it('marks invalid YAML extraction as unsuccessful', () => {
    const result = extractLocalCustomStepTypeHints(`
step_types:
  greet:
    input_schema:
      - invalid
    type: command
steps:
  - type: greet
    config:
      message: [unterminated
`);

    expect(result.ok).toBe(false);
    expect(result.stepTypes).toEqual([]);
  });
});
