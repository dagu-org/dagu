/**
 * Utilities for navigating and resolving JSON Schema paths
 */

export interface JSONSchema {
  $ref?: string;
  type?: string | string[];
  description?: string;
  default?: unknown;
  enum?: unknown[];
  examples?: unknown[];
  deprecated?: boolean;
  properties?: Record<string, JSONSchema>;
  additionalProperties?: boolean | JSONSchema;
  items?: JSONSchema | JSONSchema[];
  oneOf?: JSONSchema[];
  anyOf?: JSONSchema[];
  allOf?: JSONSchema[];
  required?: string[];
  definitions?: Record<string, JSONSchema>;
  title?: string;
  minimum?: number;
  maximum?: number;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  format?: string;
}

export interface SchemaPropertyInfo {
  name: string;
  path: string[];
  type: string | string[];
  description?: string;
  default?: unknown;
  enum?: unknown[];
  required: boolean;
  deprecated?: boolean;
  examples?: unknown[];
  properties?: Record<string, SchemaPropertyInfo>;
  items?: SchemaPropertyInfo;
  oneOf?: SchemaPropertyInfo[];
  title?: string;
  format?: string;
  pattern?: string;
}

/**
 * Resolves a schema at a given YAML path
 */
export function getSchemaAtPath(
  schema: JSONSchema,
  path: string[]
): JSONSchema | null {
  if (!schema || path.length === 0) {
    return schema;
  }

  let current: JSONSchema | null = schema;
  let currentRequired: string[] = schema.required || [];

  for (let i = 0; i < path.length; i++) {
    const segment = path[i];

    if (!current || !segment) {
      return null;
    }

    // Handle array index access
    if (/^\d+$/.test(segment)) {
      current = resolveArrayItems(current);
      if (current) {
        currentRequired = current.required || [];
      }
      continue;
    }

    // Try to resolve from properties
    const prop: JSONSchema | undefined = current.properties?.[segment];
    if (prop) {
      currentRequired = current.required || [];
      current = prop;
      continue;
    }

    // Try to find in oneOf/anyOf schemas
    const unionMatch = findInUnionTypes(current, segment);
    if (unionMatch) {
      currentRequired = unionMatch.required || [];
      const unionProp = unionMatch.properties?.[segment];
      current = unionProp || unionMatch;
      continue;
    }

    // Try allOf - merge all schemas and look for property
    if (current.allOf) {
      const allOfMatch = findInAllOf(current.allOf, segment);
      if (allOfMatch) {
        currentRequired = allOfMatch.required || [];
        const allOfProp = allOfMatch.properties?.[segment];
        current = allOfProp || allOfMatch;
        continue;
      }
    }

    // Check additionalProperties
    if (current.additionalProperties && typeof current.additionalProperties === 'object') {
      current = current.additionalProperties;
      currentRequired = current.required || [];
      continue;
    }

    // Not found
    return null;
  }

  return current;
}

/**
 * Resolves array items schema, handling oneOf arrays
 */
function resolveArrayItems(schema: JSONSchema): JSONSchema | null {
  if (!schema.items) {
    return null;
  }

  if (Array.isArray(schema.items)) {
    return schema.items[0] || null;
  }

  return schema.items;
}

/**
 * Searches oneOf/anyOf schemas for a property
 */
function findInUnionTypes(schema: JSONSchema, propertyName: string): JSONSchema | null {
  const unions = [...(schema.oneOf || []), ...(schema.anyOf || [])];

  for (const variant of unions) {
    if (variant.properties?.[propertyName]) {
      return variant;
    }
    // Recursively check nested unions
    if (variant.oneOf || variant.anyOf) {
      const nested = findInUnionTypes(variant, propertyName);
      if (nested) return nested;
    }
  }

  return null;
}

/**
 * Searches allOf schemas for a property
 */
function findInAllOf(allOf: JSONSchema[], propertyName: string): JSONSchema | null {
  for (const schema of allOf) {
    if (schema.properties?.[propertyName]) {
      return schema;
    }
    // Check nested structures
    if (schema.oneOf || schema.anyOf) {
      const nested = findInUnionTypes(schema, propertyName);
      if (nested) return nested;
    }
    if (schema.allOf) {
      const nested = findInAllOf(schema.allOf, propertyName);
      if (nested) return nested;
    }
  }
  return null;
}

/**
 * Converts a JSON Schema to SchemaPropertyInfo for display
 */
export function toPropertyInfo(
  schema: JSONSchema | null,
  name: string,
  path: string[],
  parentRequired: string[] = []
): SchemaPropertyInfo | null {
  if (!schema) {
    return null;
  }

  const typeValue = resolveType(schema);
  const isRequired = parentRequired.includes(name);

  const info: SchemaPropertyInfo = {
    name,
    path,
    type: typeValue,
    description: schema.description,
    default: schema.default,
    enum: schema.enum,
    required: isRequired,
    deprecated: schema.deprecated,
    examples: schema.examples,
    title: schema.title,
    format: schema.format,
    pattern: schema.pattern,
  };

  // Add nested properties info
  if (schema.properties) {
    info.properties = {};
    const childRequired = schema.required || [];
    for (const [key, propSchema] of Object.entries(schema.properties)) {
      const childInfo = toPropertyInfo(propSchema, key, [...path, key], childRequired);
      if (childInfo) {
        info.properties[key] = childInfo;
      }
    }
  }

  // Add items info for arrays
  if (schema.items && !Array.isArray(schema.items)) {
    info.items = toPropertyInfo(schema.items, 'items', [...path, '[]'], []) || undefined;
  }

  // Add oneOf info
  if (schema.oneOf) {
    info.oneOf = schema.oneOf
      .map((variant, i) => toPropertyInfo(variant, `option${i + 1}`, path, []))
      .filter((v): v is SchemaPropertyInfo => v !== null);
  }

  return info;
}

/**
 * Resolves the type string(s) from a schema
 */
function resolveType(schema: JSONSchema): string | string[] {
  if (schema.type) {
    return schema.type;
  }

  if (schema.oneOf) {
    const types = schema.oneOf
      .map((s) => resolveType(s))
      .flat()
      .filter((t): t is string => typeof t === 'string')
      .filter((t, i, arr) => arr.indexOf(t) === i);
    return types.length === 1 ? (types[0] ?? 'unknown') : types;
  }

  if (schema.anyOf) {
    const types = schema.anyOf
      .map((s) => resolveType(s))
      .flat()
      .filter((t): t is string => typeof t === 'string')
      .filter((t, i, arr) => arr.indexOf(t) === i);
    return types.length === 1 ? (types[0] ?? 'unknown') : types;
  }

  if (schema.enum) {
    return 'enum';
  }

  if (schema.properties) {
    return 'object';
  }

  if (schema.items) {
    return 'array';
  }

  return 'unknown';
}

/**
 * Gets sibling properties at the same level
 */
export function getSiblingProperties(
  schema: JSONSchema,
  path: string[]
): string[] {
  if (path.length === 0) {
    return Object.keys(schema.properties || {});
  }

  const parentPath = path.slice(0, -1);
  const parentSchema = getSchemaAtPath(schema, parentPath);

  if (!parentSchema?.properties) {
    return [];
  }

  return Object.keys(parentSchema.properties);
}

/**
 * Gets the parent schema's required fields to determine if current property is required
 */
export function getParentRequired(
  schema: JSONSchema,
  path: string[]
): string[] {
  if (path.length === 0) {
    return schema.required || [];
  }

  const parentPath = path.slice(0, -1);
  const parentSchema = getSchemaAtPath(schema, parentPath);

  return parentSchema?.required || [];
}
