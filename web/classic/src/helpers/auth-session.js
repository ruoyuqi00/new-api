function isClassicLoginSession(value) {
  return (
    value &&
    typeof value === 'object' &&
    typeof value.sid === 'string' &&
    value.sid.length > 0 &&
    value.current === true &&
    typeof value.login_method === 'string' &&
    typeof value.ip === 'string' &&
    typeof value.user_agent === 'string' &&
    typeof value.created_at === 'number' &&
    Number.isFinite(value.created_at) &&
    typeof value.last_active_at === 'number' &&
    Number.isFinite(value.last_active_at) &&
    typeof value.expires_at === 'number' &&
    Number.isFinite(value.expires_at)
  );
}

export function parseClassicAuthBundle(value) {
  if (!value || typeof value !== 'object') return null;
  const {
    access_token: accessToken,
    token_type: tokenType,
    access_expires_at: accessExpiresAt,
    session,
    user,
  } = value;
  if (
    typeof accessToken !== 'string' ||
    accessToken.length === 0 ||
    tokenType !== 'Bearer' ||
    typeof accessExpiresAt !== 'number' ||
    !Number.isFinite(accessExpiresAt) ||
    accessExpiresAt <= 0 ||
    !isClassicLoginSession(session) ||
    !user ||
    !Number.isInteger(user.id) ||
    user.id <= 0 ||
    typeof user.username !== 'string' ||
    typeof user.role !== 'number'
  ) {
    return null;
  }
  return { accessToken, sessionID: session.sid, user };
}

export function resolveClassicOAuthIntent(hasAccessToken, shouldLogout) {
  return hasAccessToken && !shouldLogout ? 'bind' : 'login';
}

export function shouldRefreshClassicOAuthSession(
  hasAccessToken,
  shouldLogout,
  hasSavedUser,
) {
  return !hasAccessToken && !shouldLogout && hasSavedUser;
}

export function isClassicTransientRefreshStatus(status) {
  return !status || status === 429 || status >= 500;
}
