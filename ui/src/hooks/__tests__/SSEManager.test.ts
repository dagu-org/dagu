import { describe, expect, it } from 'vitest';
import { buildAgentSessionTopic, endpointToTopic } from '../SSEManager';

describe('endpointToTopic', () => {
  it('maps DAG list endpoints to canonical topics', () => {
    expect(endpointToTopic('/events/dags?perPage=100&page=1')).toBe('dagslist:page=1&perPage=100');
  });

  it('maps DAG run log endpoints with query params', () => {
    expect(endpointToTopic('/events/dag-runs/mydag/run-1/logs?tail=500')).toBe(
      'dagrunlogs:mydag/run-1?tail=500'
    );
  });

  it('maps docs endpoints with decoded paths', () => {
    expect(endpointToTopic('/events/docs/runbooks/deploy%20guide')).toBe('doc:runbooks/deploy guide');
  });
});

describe('buildAgentSessionTopic', () => {
  it('builds an agent topic key', () => {
    expect(buildAgentSessionTopic('sess-123')).toBe('agent:sess-123');
  });
});
