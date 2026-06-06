import { id } from '@aspect-test/a';
import jquery from 'jquery';
import { value } from 'lib-a';
import { value as value2 } from 'lib-non-wksp';
import { name } from 'main-lib/package.json';

console.log(name, value, value2, id(), jquery);
