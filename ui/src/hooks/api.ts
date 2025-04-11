import createClient from 'openapi-fetch';
import {
  createQueryHook,
  createImmutableHook,
  createInfiniteHook,
  createMutateHook,
} from 'swr-openapi';
import { isMatch } from 'lodash-es';
import type { paths } from '../api/v2/schema';

const client = createClient<paths>({
  baseUrl: getConfig().apiURL,
});
const prefix = '/';

export const useQuery = createQueryHook(client, prefix);
export const useImmutable = createImmutableHook(client, prefix);
export const useInfinite = createInfiniteHook(client, prefix);
export const useMutate = createMutateHook(client, prefix, isMatch);
