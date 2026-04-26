// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import type { JSONSchema } from '@/lib/schema-utils';
import { parseParams } from '@/lib/parseParams';

export type ParamSchemaFormData = Record<string, unknown>;
export type ParamSchemaUiSchema = Record<string, Record<string, unknown>>;

const radioChoiceLimit = 4;

export function buildParamSchemaFormData(
  schema: JSONSchema,
  defaultParams?: string
): ParamSchemaFormData {
  if (!defaultParams) {
    return {};
  }

  const properties = schema.properties ?? {};
  const formData: ParamSchemaFormData = {};

  for (const param of parseParams(defaultParams)) {
    if (!param.Name) {
      continue;
    }

    const propertySchema = properties[param.Name];
    if (!propertySchema) {
      continue;
    }

    formData[param.Name] = coerceParamSchemaValue(param.Value, propertySchema);
  }

  return formData;
}

export function buildParamSchemaUiSchema(
  schema: JSONSchema
): ParamSchemaUiSchema {
  const uiSchema: ParamSchemaUiSchema = {};

  for (const [name, propertySchema] of Object.entries(
    schema.properties ?? {}
  )) {
    const choiceCount = getChoiceCount(propertySchema);
    if (choiceCount > 0 && choiceCount <= radioChoiceLimit) {
      uiSchema[name] = { 'ui:widget': 'radio' };
    }
  }

  return uiSchema;
}

export function stringifyParamSchemaFormData(
  formData: ParamSchemaFormData
): string {
  return JSON.stringify(formData);
}

function coerceParamSchemaValue(value: string, schema: JSONSchema): unknown {
  if (value.trim() === '') {
    return value;
  }

  switch (inferScalarType(schema)) {
    case 'integer': {
      const number = Number(value);
      return Number.isInteger(number) ? number : value;
    }
    case 'number': {
      const number = Number(value);
      return Number.isNaN(number) ? value : number;
    }
    case 'boolean':
      if (value === 'true') {
        return true;
      }
      if (value === 'false') {
        return false;
      }
      return value;
    case 'string':
    default:
      return value;
  }
}

function inferScalarType(schema: JSONSchema): string | undefined {
  if (typeof schema.type === 'string') {
    return schema.type;
  }
  if (schema.oneOf?.length) {
    for (const option of schema.oneOf) {
      if (typeof option.type === 'string') {
        return option.type;
      }
      if (option.const !== undefined) {
        return inferTypeFromValue(option.const);
      }
    }
  }
  if (schema.enum?.length) {
    return inferTypeFromValue(schema.enum[0]);
  }
  return undefined;
}

function inferTypeFromValue(value: unknown): string | undefined {
  switch (typeof value) {
    case 'string':
      return 'string';
    case 'boolean':
      return 'boolean';
    case 'number':
      return Number.isInteger(value) ? 'integer' : 'number';
    default:
      return undefined;
  }
}

function getChoiceCount(schema: JSONSchema): number {
  if (Array.isArray(schema.enum)) {
    return schema.enum.length;
  }
  if (Array.isArray(schema.oneOf)) {
    return schema.oneOf.filter((option) => option.const !== undefined).length;
  }
  return 0;
}
