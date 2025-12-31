import { useMemo } from 'react';
import { useSchema } from '@/contexts/SchemaContext';
import {
  getSchemaAtPath,
  toPropertyInfo,
  getParentRequired,
  getSiblingProperties,
  type SchemaPropertyInfo,
} from '@/lib/schema-utils';

export interface SchemaLookupResult {
  propertyInfo: SchemaPropertyInfo | null;
  siblingProperties: string[];
  loading: boolean;
  error: Error | null;
}

/**
 * Hook to look up schema information for a given YAML path
 */
export function useSchemaLookup(path: string[]): SchemaLookupResult {
  const { schema, loading, error } = useSchema();

  console.log('useSchemaLookup - called with path:', path, 'schema exists:', !!schema);

  const result = useMemo(() => {
    console.log('useSchemaLookup useMemo - path:', path, 'path.length:', path.length, 'schema:', !!schema);
    if (!schema || path.length === 0) {
      console.log('useSchemaLookup - early return, no schema or empty path');
      return {
        propertyInfo: null,
        siblingProperties: [],
      };
    }

    const schemaAtPath = getSchemaAtPath(schema, path);
    console.log('useSchemaLookup - schemaAtPath:', schemaAtPath);
    const parentRequired = getParentRequired(schema, path);
    const currentKey = path[path.length - 1] ?? '';
    const propertyInfo = toPropertyInfo(schemaAtPath, currentKey, path, parentRequired);
    const siblingProperties = getSiblingProperties(schema, path);

    return {
      propertyInfo,
      siblingProperties,
    };
  }, [schema, path]);

  return {
    ...result,
    loading,
    error,
  };
}

/**
 * Hook to get all root-level properties from the schema
 */
export function useRootSchemaProperties(): {
  properties: string[];
  loading: boolean;
  error: Error | null;
} {
  const { schema, loading, error } = useSchema();

  const properties = useMemo(() => {
    if (!schema?.properties) {
      return [];
    }
    return Object.keys(schema.properties);
  }, [schema]);

  return { properties, loading, error };
}
