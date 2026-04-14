import { hello } from './lib/dist/lib';
import type { Hello } from './lib/types/lib';

import { hello2 } from './lib2/dist/lib2';
import type { Hello2 } from './lib2/types/lib2';

console.log(hello satisfies Hello);
console.log(hello2 satisfies Hello2);
