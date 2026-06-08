// Self-reference resolved via the nested package.json's 'exports',
// which shadows the ancestor workspace package's scope.
import { subId } from '@nested/feature/sub';

export const id = () => subId();
