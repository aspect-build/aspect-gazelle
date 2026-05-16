import { User } from '../proto/lib_pb';

export function makeUser(name: string): User {
  return { name } as User;
}
