import { User } from '../proto/lib_pb.js';

export function makeUser(name) {
  const u = new User();
  u.setName(name);
  return u;
}
