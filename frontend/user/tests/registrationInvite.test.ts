import assert from 'node:assert/strict'
import test from 'node:test'

import { inviteCodeFromRegistrationHash } from '../src/utils/registrationInvite.ts'

test('reads a URL-encoded partner invite from the registration fragment', () => {
  assert.equal(inviteCodeFromRegistrationHash('#invite=LSRAI-a%2Bb%2Fc'), 'LSRAI-a+b/c')
})

test('rejects unrelated, malformed, and oversized fragments', () => {
  assert.equal(inviteCodeFromRegistrationHash('#section'), '')
  assert.equal(inviteCodeFromRegistrationHash('#invite=%E0%A4%A'), '')
  assert.equal(inviteCodeFromRegistrationHash(`#invite=${'a'.repeat(1025)}`), '')
})
