import type {
  Asset,
  AuthTokens,
  Comment,
  Page,
  Post,
  PrivateUser,
  ProblemDetail,
  PublicUser,
  SearchResultPage,
  Share,
  VerifyResponse,
} from '../types'

const API_BASE = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/+$/, '')
const MAX_UPLOAD_BYTES = 5 * 1024 * 1024
const ALLOWED_IMAGE_TYPES = new Set(['image/jpeg', 'image/png', 'image/webp'])

let accessToken: string | null = null
let refreshFlight: Promise<string | null> | null = null
let onSessionExpired: (() => void) | null = null

export class ApiError extends Error {
  readonly status: number
  readonly requestId?: string

  constructor(status: number, message: string, requestId?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.requestId = requestId
  }
}

export function setAccessToken(token: string | null): void {
  accessToken = token
}

export function setSessionExpiredHandler(handler: (() => void) | null): void {
  onSessionExpired = handler
}

function tokenFrom(tokens: AuthTokens): string | null {
  return tokens.accessToken ?? tokens.access_token ?? null
}

async function decodeError(response: Response): Promise<ApiError> {
  let problem: ProblemDetail | undefined
  try {
    problem = (await response.json()) as ProblemDetail
  } catch {
    // Non-JSON errors are reported with the HTTP status below.
  }

  return new ApiError(
    response.status,
    problem?.detail || problem?.title || `Request failed with status ${response.status}.`,
    problem?.requestId,
  )
}

async function refreshOnce(notifyExpired = true): Promise<string | null> {
  if (!refreshFlight) {
    refreshFlight = (async () => {
      const response = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
        body: JSON.stringify({}),
        credentials: 'include',
      })

      if (response.status === 401 || response.status === 422) {
        accessToken = null
        if (notifyExpired) onSessionExpired?.()
        return null
      }
      if (!response.ok) throw await decodeError(response)

      const tokens = (await response.json()) as AuthTokens
      const newToken = tokenFrom(tokens)
      if (!newToken) {
        throw new Error('The refresh response did not contain an access token.')
      }
      accessToken = newToken
      return newToken
    })().finally(() => {
      refreshFlight = null
    })
  }
  return refreshFlight
}

async function request<T>(
  path: string,
  init: RequestInit = {},
  retryUnauthorized = true,
): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Accept', 'application/json')
  if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (accessToken && !headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${accessToken}`)
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers,
    credentials: 'include',
  })

  if (response.status === 401 && retryUnauthorized && accessToken && !path.startsWith('/auth/')) {
    const newToken = await refreshOnce()
    if (newToken) return request<T>(path, init, false)
  }

  if (!response.ok) throw await decodeError(response)
  if (response.status === 204) return undefined as T
  return (await response.json()) as T
}

function queryString(params: Record<string, string | number | undefined>): string {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined) query.set(key, String(value))
  }
  const serialized = query.toString()
  return serialized ? `?${serialized}` : ''
}

export const api = {
  requestCode(email: string): Promise<{ message?: string }> {
    return request('/auth/request-code', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }, false)
  },

  verifyCode(email: string, code: string): Promise<VerifyResponse> {
    return request('/auth/verify', {
      method: 'POST',
      body: JSON.stringify({ email, code }),
    }, false)
  },

  signup(
    signupToken: string,
    input: { username: string; displayName: string; birthdate: string },
  ): Promise<AuthTokens & { user: PrivateUser }> {
    return request('/auth/signup', {
      method: 'POST',
      headers: { Authorization: `Bearer ${signupToken}` },
      body: JSON.stringify(input),
    }, false)
  },

  async refresh(): Promise<string | null> {
    return refreshOnce(false)
  },

  logout(): Promise<void> {
    return request('/auth/logout', { method: 'POST', body: JSON.stringify({}) }, false)
  },

  getMe(): Promise<PrivateUser> {
    return request('/me')
  },

  getProfile(username: string): Promise<PublicUser> {
    return request(`/users/${encodeURIComponent(username)}`)
  },

  updateMe(input: { displayName?: string; avatarAssetId?: string | null }): Promise<PrivateUser> {
    return request('/me', { method: 'PATCH', body: JSON.stringify(input) })
  },

  getFeed(cursor?: string | null): Promise<Page<Post>> {
    return request(`/feed${queryString({ cursor: cursor ?? undefined, limit: 20 })}`)
  },

  getUserPosts(userId: string, cursor?: string | null): Promise<Page<Post>> {
    return request(`/users/${encodeURIComponent(userId)}/posts${queryString({ cursor: cursor ?? undefined, limit: 20 })}`)
  },

  getPost(postId: string): Promise<Post> {
    return request(`/posts/${encodeURIComponent(postId)}`)
  },

  createPost(input: { body: string; assetIds?: string[] }): Promise<Post> {
    return request('/posts', { method: 'POST', body: JSON.stringify(input) })
  },

  updatePost(postId: string, body: string): Promise<Post> {
    return request(`/posts/${encodeURIComponent(postId)}`, {
      method: 'PATCH',
      body: JSON.stringify({ body }),
    })
  },

  deletePost(postId: string): Promise<void> {
    return request(`/posts/${encodeURIComponent(postId)}`, { method: 'DELETE' })
  },

  likePost(postId: string, liked: boolean): Promise<{ liked: boolean }> {
    return request(`/posts/${encodeURIComponent(postId)}/like`, { method: liked ? 'PUT' : 'DELETE' })
  },

  sharePost(postId: string, quote: string): Promise<Share> {
    return request(`/posts/${encodeURIComponent(postId)}/share`, {
      method: 'POST',
      body: JSON.stringify({ quote: quote || null }),
    })
  },

  getComments(postId: string, cursor?: string | null): Promise<Page<Comment>> {
    return request(`/posts/${encodeURIComponent(postId)}/comments${queryString({ cursor: cursor ?? undefined, limit: 20 })}`)
  },

  createComment(postId: string, body: string): Promise<Comment> {
    return request(`/posts/${encodeURIComponent(postId)}/comments`, {
      method: 'POST',
      body: JSON.stringify({ body }),
    })
  },

  deleteComment(commentId: string): Promise<void> {
    return request(`/comments/${encodeURIComponent(commentId)}`, { method: 'DELETE' })
  },

  search(q: string, type: 'users' | 'posts' | 'comments', cursor?: string | null): Promise<SearchResultPage> {
    return request(`/search${queryString({ q, type, cursor: cursor ?? undefined, limit: 20 })}`)
  },

  async follow(userId: string, following: boolean): Promise<{ following: boolean }> {
    return request(`/users/${encodeURIComponent(userId)}/follow`, {
      method: following ? 'PUT' : 'DELETE',
    })
  },

  getFollowers(userId: string, cursor?: string | null): Promise<Page<PublicUser>> {
    return request(`/users/${encodeURIComponent(userId)}/followers${queryString({ cursor: cursor ?? undefined, limit: 20 })}`)
  },

  getFollowing(userId: string, cursor?: string | null): Promise<Page<PublicUser>> {
    return request(`/users/${encodeURIComponent(userId)}/following${queryString({ cursor: cursor ?? undefined, limit: 20 })}`)
  },

  async uploadImage(file: File): Promise<Asset> {
    if (!ALLOWED_IMAGE_TYPES.has(file.type)) {
      throw new Error('Choose a JPEG, PNG, or WebP image.')
    }
    if (file.size < 1 || file.size > MAX_UPLOAD_BYTES) {
      throw new Error('Images must be between 1 byte and 5 MB.')
    }

    const created = await request<{ assetId: string; presignedPutUrl: string }>('/assets', {
      method: 'POST',
      body: JSON.stringify({ mimeType: file.type, sizeBytes: file.size }),
    })

    let uploadResponse: Response
    try {
      uploadResponse = await fetch(created.presignedPutUrl, {
        method: 'PUT',
        headers: { 'Content-Type': file.type },
        body: file,
        credentials: 'omit',
      })
    } catch (error) {
      throw new Error(
        `Direct image upload failed. Check object-storage CORS and the signed PUT requirements. ${
          error instanceof Error ? error.message : 'Network error.'
        }`,
      )
    }
    if (!uploadResponse.ok) {
      throw await decodeError(uploadResponse)
    }

    const completed = await request<{ assetId: string; url: string }>(
      `/assets/${encodeURIComponent(created.assetId)}/complete`,
      { method: 'POST' },
    )
    return {
      id: completed.assetId,
      url: completed.url,
      mimeType: file.type,
      sizeBytes: file.size,
      status: 'ready',
    }
  },
}
