import { components } from '../../../api/v1/schema';

export function getEventHandlers(s: components['schemas']['DAGRunDetails']) {
  const ret: components['schemas']['Node'][] = [];
  if (s.onSuccess) {
    ret.push(s.onSuccess);
  }
  if (s.onFailure) {
    ret.push(s.onFailure);
  }
  if (s.onCancel) {
    ret.push(s.onCancel);
  }
  if (s.onExit) {
    ret.push(s.onExit);
  }
  return ret;
}
