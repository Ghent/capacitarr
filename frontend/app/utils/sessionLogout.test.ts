import { describe, it, expect, vi, beforeEach } from 'vitest';

const { mockOfetch } = vi.hoisted(() => ({
  mockOfetch: vi.fn(),
}));

vi.mock('ofetch', () => ({
  ofetch: mockOfetch,
}));

import { logoutSession } from './sessionLogout';

describe('logoutSession', () => {
  beforeEach(() => {
    mockOfetch.mockReset();
    mockOfetch.mockResolvedValue(undefined);
  });

  it('POSTs /auth/logout with credentials included', async () => {
    await logoutSession('http://localhost:2187');

    expect(mockOfetch).toHaveBeenCalledTimes(1);
    expect(mockOfetch).toHaveBeenCalledWith('http://localhost:2187/api/v1/auth/logout', {
      method: 'POST',
      credentials: 'include',
    });
  });
});
