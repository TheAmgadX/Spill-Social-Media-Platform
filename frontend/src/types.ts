export interface Pagination {
  nextCursor: string | null
  limit: number
  hasMore: boolean
}

export interface Page<T> {
  data: T[]
  pagination: Pagination
}

export interface PublicUser {
  id: string
  username: string
  displayName: string
  avatarUrl: string | null
  role: 'user' | 'admin' | string
  followerCount: number
  followingCount: number
  createdAt: string
}

export interface PrivateUser extends PublicUser {
  email: string
  birthdate: string
}

export interface Asset {
  id: string
  url: string
  mimeType: string
  sizeBytes: number
  status: 'pending' | 'ready' | string
}

export interface Post {
  id: string
  authorId: string
  author?: PublicUser
  body: string
  assets?: Asset[]
  likeCount: number
  commentCount: number
  shareCount: number
  liked: boolean | null
  createdAt: string
}

export interface Comment {
  id: string
  postId: string
  authorId: string
  author?: PublicUser
  body: string
  createdAt: string
}

export interface Share {
  id: string
  userId: string
  postId: string
  quote: string | null
  createdAt: string
}

export interface AuthTokens {
  accessToken?: string
  access_token?: string
  refreshToken?: string
  refresh_token?: string
}

export interface VerifyResponse extends AuthTokens {
  action: 'login' | 'signup'
  new_user?: boolean
  signupToken?: string
  signup_token?: string
}

export interface ProblemDetail {
  type?: string
  title?: string
  status?: number
  detail?: string
  requestId?: string
}

export interface SearchResultPage {
  data: PublicUser[] | Post[] | Comment[]
  pagination: Pagination
}
