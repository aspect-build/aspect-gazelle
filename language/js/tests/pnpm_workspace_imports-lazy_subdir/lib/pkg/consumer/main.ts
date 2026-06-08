// This file lives in a SUBDIRECTORY of the @lib/pkg pnpm project. Its '#' subpath
// imports are resolved by the project's lib/pkg/package.json (not lib/pkg/consumer),
// so lazy indexing must consult the project root's 'imports' to index the targets.
import { subId } from '#sub';
import { internalId } from '#internal/util';
import { dbId } from '#~db';

export const consumerId = () => subId() + internalId() + dbId();
