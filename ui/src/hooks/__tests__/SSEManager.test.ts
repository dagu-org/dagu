import { describe, expect, it } from 'vitest';
import { buildAgentSessionTopic, endpointToTopic } from '../SSEManager';

describe('endpointToTopic', () => {
  it('maps every supported legacy SSE endpoint to a canonical topic', () => {
    const cases: Array<[string, string]> = [
      ['/events/dags?perPage=100&page=1', 'dagslist:page=1&perPage=100'],
      ['/events/dags/example.yaml', 'dag:example.yaml'],
      ['/events/dags/example.yaml/dag-runs', 'daghistory:example.yaml'],
      [
        '/events/dag-runs?page=2&status=running',
        'dagruns:page=2&status=running',
      ],
      ['/events/dag-runs/mydag/run-1', 'dagrun:mydag/run-1'],
      [
        '/events/dag-runs/mydag/run-1/logs?tail=500',
        'dagrunlogs:mydag/run-1?tail=500',
      ],
      [
        '/events/dag-runs/mydag/run-1/logs/steps/build',
        'steplog:mydag/run-1/build',
      ],
      ['/events/queues?status=active&page=3', 'queues:page=3&status=active'],
      ['/events/queues/default/items', 'queueitems:default'],
      ['/events/docs-tree?prefix=guides', 'doctree:prefix=guides'],
      ['/events/docs/runbooks/deploy%20guide', 'doc:runbooks/deploy guide'],
    ];

    for (const [endpoint, topic] of cases) {
      expect(endpointToTopic(endpoint)).toBe(topic);
    }
  });
});

describe('buildAgentSessionTopic', () => {
  it('builds an agent topic key', () => {
    expect(buildAgentSessionTopic('sess-123')).toBe('agent:sess-123');
  });
});
