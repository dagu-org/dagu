import createClient, { Middleware } from 'openapi-fetch';
import {
  createQueryHook,
  createImmutableHook,
  createInfiniteHook,
  createMutateHook,
} from 'swr-openapi';
import { isMatch } from 'lodash-es';
import type { paths } from '../api/v1/schema';

const authMiddleware: Middleware = {
  async onRequest({ request }) {
    const token = localStorage.getItem('dagu_auth_token');
    if (token) {
      request.headers.set('Authorization', `Bearer ${token}`);
    }
    return request;
  },
};

const client = createClient<paths>({
  baseUrl: getConfig().apiURL,
});
client.use(authMiddleware);

const prefix = '/';

export const useQuery = createQueryHook(client, prefix);
export const useImmutable = createImmutableHook(client, prefix);
export const useInfinite = createInfiniteHook(client, prefix);
export const useMutate = createMutateHook(client, prefix, isMatch);
export const useClient = () => client;
