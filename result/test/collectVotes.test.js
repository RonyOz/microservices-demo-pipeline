const test = require('node:test');
const assert = require('node:assert/strict');

const { collectVotesFromResult } = require('../server');

test('collectVotesFromResult maps row counts into vote totals', () => {
  const result = {
    rows: [
      { vote: 'a', count: '3' },
      { vote: 'b', count: '7' },
    ],
  };

  assert.deepEqual(collectVotesFromResult(result), { a: 3, b: 7 });
});

test('collectVotesFromResult defaults missing options to zero', () => {
  const result = {
    rows: [{ vote: 'a', count: '2' }],
  };

  assert.deepEqual(collectVotesFromResult(result), { a: 2, b: 0 });
});
