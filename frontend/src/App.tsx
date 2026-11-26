import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from 'react'
import {
  BrowserRouter,
  Link,
  NavLink,
  Route,
  Routes,
  useNavigate,
  useParams,
} from 'react-router-dom'
import { Avatar } from './components/Avatar'
import { Icon } from './components/Icon'
import { PostCard } from './components/PostCard'
import { api } from './services/api'
import { AuthProvider } from './services/auth'
import { useAuth } from './services/useAuth'
import type { Asset, Comment, Post, PrivateUser, PublicUser, SearchResultPage, VerifyResponse } from './types'

interface StreamState<T> {
  items: T[]
  cursor: string | null
  hasMore: boolean
  loading: boolean
  error: string | null
}

function emptyStream<T>(): StreamState<T> {
  return { items: [], cursor: null, hasMore: false, loading: true, error: null }
}

function mergeUnique<T extends { id: string }>(existing: T[], incoming: T[]): T[] {
  const known = new Set(existing.map((item) => item.id))
  const additions = incoming.filter((item) => {
    if (known.has(item.id)) return false
    known.add(item.id)
    return true
  })
  return [...existing, ...additions]
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}

function AppFrame() {
  const { user, loading, notice, dismissNotice } = useAuth()

  if (loading) {
    return <main className="boot-screen"><span className="spinner" />Restoring your session…</main>
  }
  if (!user) return <AuthPage bootNotice={notice} />

  return (
    <div className="app-shell">
      <header className="mobile-header">
        <Link to="/" className="wordmark"><span className="wordmark-dot" />spill</Link>
        <Link to={`/u/${encodeURIComponent(user.username)}`} aria-label="Your profile"><Avatar user={user} size="small" /></Link>
      </header>
      <aside className="side-rail">
        <Link to="/" className="wordmark"><span className="wordmark-dot" />spill</Link>
        <nav className="primary-nav" aria-label="Main navigation">
          <NavLink to="/" end className={({ isActive }) => `nav-item ${isActive ? 'is-active' : ''}`}><Icon name="home" /><span>Home</span></NavLink>
          <NavLink to="/search" className={({ isActive }) => `nav-item ${isActive ? 'is-active' : ''}`}><Icon name="search" /><span>Search</span></NavLink>
          <NavLink to={`/u/${encodeURIComponent(user.username)}`} className={({ isActive }) => `nav-item ${isActive ? 'is-active' : ''}`}><Icon name="user" /><span>Profile</span></NavLink>
        </nav>
        <div className="rail-note">
          <Icon name="spark" size={17} />
          <p>A little more signal.<br />A little less noise.</p>
        </div>
        <div className="rail-profile">
          <Avatar user={user} size="small" />
          <span className="rail-profile-copy"><strong>{user.displayName}</strong><small>@{user.username}</small></span>
          <SignOutButton />
        </div>
      </aside>

      <div className="workspace">
        {notice && (
          <div className="session-notice" role="status">
            <span>{notice}</span>
            <button type="button" className="icon-button" aria-label="Dismiss notice" onClick={dismissNotice}><Icon name="close" size={17} /></button>
          </div>
        )}
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/search" element={<SearchPage />} />
          <Route path="/u/:username" element={<ProfilePage />} />
          <Route path="/p/:postId" element={<PostDetailPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </div>

      <aside className="right-rail">
        <div className="right-card account-card">
          <div className="right-card-heading">Your corner</div>
          <div className="account-summary">
            <Avatar user={user} size="large" />
            <div><strong>{user.displayName}</strong><small>@{user.username}</small></div>
          </div>
          <div className="account-stats">
            <span><strong>{user.followerCount}</strong> followers</span>
            <span><strong>{user.followingCount}</strong> following</span>
          </div>
          <Link className="text-link" to={`/u/${encodeURIComponent(user.username)}`}>View your profile <Icon name="arrow-right" size={15} /></Link>
        </div>
        <div className="right-card about-card">
          <div className="about-mark"><Icon name="spark" size={18} /></div>
          <p>Make room for the things worth saying.</p>
          <small>Spill · a social space for your point of view</small>
        </div>
        <footer className="rail-footer">Your words, your space. <span>© Spill</span></footer>
      </aside>

      <nav className="mobile-nav" aria-label="Mobile navigation">
        <NavLink to="/" end aria-label="Home"><Icon name="home" /></NavLink>
        <NavLink to="/search" aria-label="Search"><Icon name="search" /></NavLink>
        <NavLink to={`/u/${encodeURIComponent(user.username)}`} aria-label="Profile"><Icon name="user" /></NavLink>
      </nav>
    </div>
  )
}

function SignOutButton() {
  const { signOut } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  async function logout() {
    setError(null)
    try {
      await signOut()
      navigate('/')
    } catch (reason) {
      setError(messageOf(reason, 'Could not complete sign out.'))
    }
  }

  return (
    <>
      <button type="button" className="icon-button signout-button" onClick={() => void logout()} aria-label="Sign out" title="Sign out">
        <Icon name="logout" size={18} />
      </button>
      {error && <span className="sr-only" role="alert">{error}</span>}
    </>
  )
}

function AuthPage({ bootNotice }: { bootNotice: string | null }) {
  const { establishSession } = useAuth()
  const [step, setStep] = useState<'email' | 'code' | 'signup'>('email')
  const [email, setEmail] = useState('')
  const [code, setCode] = useState('')
  const [signupToken, setSignupToken] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [username, setUsername] = useState('')
  const [birthdate, setBirthdate] = useState('')
  const [notice, setNotice] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(bootNotice)
  const [busy, setBusy] = useState(false)

  async function requestCode(event?: FormEvent) {
    event?.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const response = await api.requestCode(email.trim())
      setError(null)
      setNotice(response.message ?? 'If the email is valid, a login code has been sent.')
      setStep('code')
    } catch (reason) {
      setError(messageOf(reason, 'Could not request a sign-in code.'))
    } finally {
      setBusy(false)
    }
  }

  async function verifyCode(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const result: VerifyResponse = await api.verifyCode(email.trim(), code.trim())
      if (result.action === 'signup') {
        const token = result.signupToken ?? result.signup_token
        if (!token) throw new Error('Verification succeeded but the signup token is missing.')
        setSignupToken(token)
        setStep('signup')
      } else {
        const token = result.accessToken ?? result.access_token
        if (!token) throw new Error('Sign-in succeeded but the access token is missing.')
        await establishSession(token)
      }
    } catch (reason) {
      setError(messageOf(reason, 'Could not verify the code.'))
    } finally {
      setBusy(false)
    }
  }

  async function finishSignup(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const tokens = await api.signup(signupToken, { username: username.trim(), displayName: displayName.trim(), birthdate })
      const token = tokens.accessToken ?? tokens.access_token
      if (!token) throw new Error('Your account was created but the access token is missing.')
      await establishSession(token)
    } catch (reason) {
      setError(messageOf(reason, 'Could not complete registration.'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-art" aria-label="Spill introduction">
        <Link to="/" className="wordmark wordmark--light"><span className="wordmark-dot" />spill</Link>
        <div className="art-copy">
          <span className="eyebrow eyebrow--light">A place for your point of view</span>
          <h1>Say what<br /><em>you mean.</em></h1>
          <p>Find your people. Share the thought that won't leave you alone.</p>
          <div className="art-stamp"><span>SPILL</span><small>EST. HERE & NOW</small></div>
        </div>
        <div className="art-foot"><span>For the curious, the candid, the in-between.</span><span>01 — 01</span></div>
        <span className="art-orbit art-orbit--one" />
        <span className="art-orbit art-orbit--two" />
      </section>

      <section className="auth-panel">
        <div className="auth-panel-top"><span>Already a member?</span><span className="auth-status">✳ <small>Open for conversation</small></span></div>
        <div className="auth-form-wrap">
          <span className="eyebrow">WELCOME TO SPILL</span>
          <h2>{step === 'email' ? 'A fresh start.' : step === 'code' ? 'Check your inbox.' : 'Make it yours.'}</h2>
          <p className="auth-intro">
            {step === 'email' && 'Sign in or create an account with your email. No password to remember.'}
            {step === 'code' && `We sent a one-time code to ${email}.`}
            {step === 'signup' && 'One last step before you find your people.'}
          </p>

          {notice && <div className="form-notice" role="status">{notice}</div>}
          {error && <div className="form-error" role="alert">{error}</div>}

          {step === 'email' && (
            <form className="auth-form" onSubmit={(event) => void requestCode(event)}>
              <label>Email address<input type="email" autoComplete="email" value={email} onChange={(event) => setEmail(event.target.value)} required maxLength={254} placeholder="you@example.com" /></label>
              <button className="button button--primary button--wide" disabled={busy}>{busy ? 'Sending code…' : 'Continue with email'} <Icon name="arrow-right" size={17} /></button>
              <p className="auth-legal">By continuing, you agree to keep it thoughtful and kind.</p>
            </form>
          )}

          {step === 'code' && (
            <form className="auth-form" onSubmit={(event) => void verifyCode(event)}>
              <label>Six-digit code<input className="otp-input" inputMode="numeric" autoComplete="one-time-code" pattern="[0-9]{6}" maxLength={6} value={code} onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))} required placeholder="000000" /></label>
              <button className="button button--primary button--wide" disabled={busy || code.length !== 6}>{busy ? 'Verifying…' : 'Verify code'} <Icon name="arrow-right" size={17} /></button>
              <div className="auth-secondary"><button type="button" className="text-button" onClick={() => { setStep('email'); setNotice(null); setError(null) }}>Use a different email</button><button type="button" className="text-button" disabled={busy} onClick={() => void requestCode()}>Send a new code</button></div>
            </form>
          )}

          {step === 'signup' && (
            <form className="auth-form" onSubmit={(event) => void finishSignup(event)}>
              <label>Display name<input value={displayName} onChange={(event) => setDisplayName(event.target.value)} required maxLength={50} /></label>
              <label>Username<div className="input-prefix"><span>@</span><input value={username} onChange={(event) => setUsername(event.target.value.toLowerCase().replace(/[^a-z0-9_]/g, '').slice(0, 20))} required minLength={3} maxLength={20} pattern="[a-z0-9_]{3,20}" /></div></label>
              <label>Birth date<input type="date" value={birthdate} onChange={(event) => setBirthdate(event.target.value)} required /></label>
              <button className="button button--primary button--wide" disabled={busy}>{busy ? 'Creating account…' : 'Create my account'} <Icon name="arrow-right" size={17} /></button>
            </form>
          )}
        </div>
        <div className="auth-panel-foot"><span>Good conversations start somewhere.</span><span>© Spill Social</span></div>
      </section>
    </main>
  )
}

function PageHeader({ eyebrow, title, trailing }: { eyebrow?: string; title: string; trailing?: ReactNode }) {
  return (
    <header className="page-header">
      <div>{eyebrow && <span className="eyebrow">{eyebrow}</span>}<h1>{title}</h1></div>
      {trailing}
    </header>
  )
}

function HomePage() {
  const { user } = useAuth()
  const [following, setFollowing] = useState<StreamState<Post>>(emptyStream())
  const [mine, setMine] = useState<StreamState<Post>>(emptyStream())

  const loadFollowing = useCallback(async (cursor: string | null = null) => {
    setFollowing((current) => ({ ...current, loading: true, error: null }))
    try {
      const page = await api.getFeed(cursor)
      setFollowing((current) => ({
        items: cursor ? mergeUnique(current.items, page.data) : page.data,
        cursor: page.pagination.nextCursor,
        hasMore: page.pagination.hasMore,
        loading: false,
        error: null,
      }))
    } catch (reason) {
      setFollowing((current) => ({ ...current, loading: false, error: messageOf(reason, 'Could not load your following feed.') }))
    }
  }, [])

  const loadMine = useCallback(async (cursor: string | null = null) => {
    if (!user) return
    setMine((current) => ({ ...current, loading: true, error: null }))
    try {
      const page = await api.getUserPosts(user.id, cursor)
      setMine((current) => ({
        items: cursor ? mergeUnique(current.items, page.data) : page.data,
        cursor: page.pagination.nextCursor,
        hasMore: page.pagination.hasMore,
        loading: false,
        error: null,
      }))
    } catch (reason) {
      setMine((current) => ({ ...current, loading: false, error: messageOf(reason, 'Could not load your posts.') }))
    }
  }, [user])

  useEffect(() => {
    void loadFollowing()
    void loadMine()
  }, [loadFollowing, loadMine])

  function updatePost(next: Post, setter: typeof setFollowing) {
    setter((current) => ({ ...current, items: current.items.map((item) => item.id === next.id ? next : item) }))
  }

  return (
    <main className="page-content">
      <PageHeader eyebrow="YOUR DAILY READ" title="Home" trailing={<span className="date-stamp">{new Date().toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}</span>} />
      <PostComposer user={user} onCreated={(post) => setMine((current) => ({ ...current, items: mergeUnique([post], current.items) }))} />

      <section className="feed-section" aria-labelledby="following-heading">
        <div className="section-heading">
          <div><span className="eyebrow">THE PEOPLE YOU FOLLOW</span><h2 id="following-heading">From your circle</h2></div>
          <span className="section-mark"><Icon name="users" size={18} /></span>
        </div>
        <StreamContent
          stream={following}
          onRetry={() => void loadFollowing()}
          onMore={() => void loadFollowing(following.cursor)}
          emptyTitle="A little quiet here."
          emptyCopy="When people you follow share a thought, it will find its way here."
          onPostChange={(post) => updatePost(post, setFollowing)}
        />
      </section>

      <section className="feed-section own-feed-section" aria-labelledby="mine-heading">
        <div className="section-heading">
          <div><span className="eyebrow">YOUR WORDS</span><h2 id="mine-heading">Your posts</h2></div>
          <span className="section-mark section-mark--warm"><Icon name="user" size={18} /></span>
        </div>
        <StreamContent
          stream={mine}
          onRetry={() => void loadMine()}
          onMore={() => void loadMine(mine.cursor)}
          emptyTitle="Your story starts here."
          emptyCopy="Write the first post you want to see on your profile."
          onPostChange={(post) => updatePost(post, setMine)}
        />
      </section>
    </main>
  )
}

function StreamContent({
  stream,
  onRetry,
  onMore,
  emptyTitle,
  emptyCopy,
  onPostChange,
}: {
  stream: StreamState<Post>
  onRetry: () => void
  onMore: () => void
  emptyTitle: string
  emptyCopy: string
  onPostChange: (post: Post) => void
}) {
  if (stream.loading && !stream.items.length) return <LoadingBlock label="Gathering the latest…" />
  if (stream.error && !stream.items.length) return <ErrorBlock message={stream.error} onRetry={onRetry} />
  if (!stream.items.length) return <EmptyBlock title={emptyTitle} copy={emptyCopy} />

  return (
    <>
      {stream.error && <ErrorBlock message={stream.error} onRetry={onRetry} compact />}
      <div className="post-list">{stream.items.map((post) => <PostCard key={post.id} post={post} onChange={onPostChange} />)}</div>
      {stream.hasMore && (
        <button type="button" className="button button--quiet load-more" onClick={onMore} disabled={stream.loading}>
          {stream.loading ? 'Loading…' : 'Load more'}
        </button>
      )}
    </>
  )
}

function PostComposer({ user, onCreated }: { user: PrivateUser | null; onCreated: (post: Post) => void }) {
  const [body, setBody] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [uploaded, setUploaded] = useState<Map<File, Asset>>(() => new Map())
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [progress, setProgress] = useState('')

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (!body.trim()) {
      setError('Write something before posting.')
      return
    }
    setUploading(true)
    setError(null)
    try {
      const assetIds: string[] = []
      for (let index = 0; index < files.length; index += 1) {
        const file = files[index]
        let asset = uploaded.get(file)
        if (!asset) {
          setProgress(`Uploading image ${index + 1} of ${files.length}…`)
          asset = await api.uploadImage(file)
          setUploaded((current) => new Map(current).set(file, asset!))
        }
        assetIds.push(asset.id)
      }
      setProgress('Publishing your post…')
      const post = await api.createPost({ body: body.trim(), ...(assetIds.length ? { assetIds } : {}) })
      onCreated(post)
      setBody('')
      setFiles([])
      setUploaded(new Map())
    } catch (reason) {
      setError(messageOf(reason, 'Could not publish this post.'))
    } finally {
      setProgress('')
      setUploading(false)
    }
  }

  return (
    <form className="composer-card" onSubmit={(event) => void submit(event)}>
      <div className="composer-top">
        <Avatar user={user} />
        <textarea value={body} onChange={(event) => setBody(event.target.value)} placeholder="What's on your mind?" aria-label="Post text" rows={3} maxLength={2000} />
      </div>
      {files.length > 0 && (
        <div className="composer-files">
          {files.map((file, index) => (
            <span key={`${file.name}-${file.lastModified}-${index}`} className="file-chip">
              <Icon name="image" size={15} />{file.name}
              {uploaded.has(file) && <span className="file-ready">Ready</span>}
              <button type="button" aria-label={`Remove ${file.name}`} onClick={() => setFiles((current) => current.filter((_, fileIndex) => fileIndex !== index))}><Icon name="close" size={14} /></button>
            </span>
          ))}
        </div>
      )}
      {error && <p className="inline-error" role="alert">{error}</p>}
      {progress && <p className="upload-progress" role="status"><span className="spinner spinner--small" />{progress}</p>}
      <div className="composer-actions">
        <label className="attach-button">
          <Icon name="image" size={18} /><span>Add image</span>
          <input
            type="file"
            accept="image/jpeg,image/png,image/webp"
            multiple
            onChange={(event) => {
              const picked = Array.from(event.target.files ?? [])
              if (files.length + picked.length > 10) {
                setError('A post can include up to 10 images. Remove an image before adding more.')
                event.target.value = ''
                return
              }
              setError(null)
              setFiles((current) => [...current, ...picked])
              event.target.value = ''
            }}
          />
        </label>
        <button type="submit" className="button button--primary button--small" disabled={uploading || !body.trim()}>
          {uploading ? 'Working…' : 'Post'} <Icon name="arrow-right" size={16} />
        </button>
      </div>
    </form>
  )
}

function SearchPage() {
  const [query, setQuery] = useState('')
  const [submitted, setSubmitted] = useState('')
  const [type, setType] = useState<'users' | 'posts' | 'comments'>('posts')
  const [result, setResult] = useState<SearchResultPage | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function search(nextCursor?: string | null) {
    if (!query.trim()) {
      setError('Enter a search term to continue.')
      return
    }
    setLoading(true)
    setError(null)
    try {
      const page = await api.search(query.trim(), type, nextCursor)
      setSubmitted(query.trim())
      setResult((current) => nextCursor && current
        ? { ...page, data: mergeSearchData(current.data, page.data) }
        : page)
    } catch (reason) {
      setError(messageOf(reason, 'Search could not be completed.'))
    } finally {
      setLoading(false)
    }
  }

  function changeType(nextType: 'users' | 'posts' | 'comments') {
    setType(nextType)
    setResult(null)
    setError(null)
  }

  return (
    <main className="page-content">
      <PageHeader eyebrow="FIND YOUR THREAD" title="Search" />
      <form className="search-form" onSubmit={(event) => { event.preventDefault(); void search() }}>
        <Icon name="search" size={21} />
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search Spill…" aria-label="Search query" maxLength={200} />
        <button className="button button--primary button--small" type="submit" disabled={loading}>{loading ? 'Searching…' : 'Search'}</button>
      </form>
      <div className="search-tabs" role="tablist" aria-label="Search category">
        {(['posts', 'users', 'comments'] as const).map((category) => (
          <button key={category} type="button" role="tab" aria-selected={type === category} className={type === category ? 'search-tab is-active' : 'search-tab'} onClick={() => changeType(category)} disabled={loading}>
            {category[0].toUpperCase()}{category.slice(1)}
          </button>
        ))}
      </div>
      {error && <ErrorBlock message={error} onRetry={() => void search()} compact={!result} />}
      {loading && !result && <LoadingBlock label="Looking around…" />}
      {result && (
        <section className="search-results">
          <div className="results-caption"><span>RESULTS FOR</span><strong>“{submitted}”</strong></div>
          {!result.data.length ? <EmptyBlock title="No matches yet." copy="Try another phrase or search in a different category." /> : (
            <div className="post-list">
              {type === 'posts' && (result.data as Post[]).map((post) => (
                <PostCard
                  key={post.id}
                  post={post}
                  onChange={(next) => setResult((current) => current
                    ? { ...current, data: (current.data as Post[]).map((item) => item.id === next.id ? next : item) }
                    : current)}
                />
              ))}
              {type === 'users' && (result.data as PublicUser[]).map((person) => <UserResult key={person.id} user={person} />)}
              {type === 'comments' && (result.data as Comment[]).map((comment) => <CommentResult key={comment.id} comment={comment} />)}
            </div>
          )}
          {result.pagination.hasMore && <button className="button button--quiet load-more" type="button" disabled={loading} onClick={() => void search(result.pagination.nextCursor)}>{loading ? 'Loading…' : 'More results'}</button>}
        </section>
      )}
    </main>
  )
}

function UserResult({ user }: { user: PublicUser }) {
  return (
    <Link className="user-result" to={`/u/${encodeURIComponent(user.username)}`}>
      <Avatar user={user} size="large" />
      <span className="user-result-copy"><strong>{user.displayName}</strong><small>@{user.username}</small></span>
      <span className="user-result-count">{user.followerCount} followers</span>
      <Icon name="arrow-right" size={17} />
    </Link>
  )
}

function CommentResult({ comment }: { comment: Comment }) {
  return (
    <Link className="comment-result" to={`/p/${encodeURIComponent(comment.postId)}`}>
      <div className="comment-result-head"><Avatar user={comment.author} size="small" /><strong>{comment.author?.displayName ?? 'Spill member'}</strong><span>@{comment.author?.username ?? 'unknown'}</span></div>
      <p>{comment.body}</p>
      <small>Open the conversation <Icon name="arrow-right" size={14} /></small>
    </Link>
  )
}

function mergeSearchData(
  existing: SearchResultPage['data'],
  incoming: SearchResultPage['data'],
): SearchResultPage['data'] {
  const known = new Set(existing.map((item) => item.id))
  const additions = incoming.filter((item) => {
    if (known.has(item.id)) return false
    known.add(item.id)
    return true
  })
  return [...existing, ...additions] as SearchResultPage['data']
}

function ProfilePage() {
  const { username = '' } = useParams()
  const { user: currentUser } = useAuth()
  const [profile, setProfile] = useState<PublicUser | null>(null)
  const [posts, setPosts] = useState<StreamState<Post>>(emptyStream())
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [followState, setFollowState] = useState<boolean | null>(null)
  const [followBusy, setFollowBusy] = useState(false)
  const [followError, setFollowError] = useState<string | null>(null)
  const ownProfile = currentUser?.username.toLowerCase() === username.toLowerCase()

  const loadProfilePosts = useCallback(async (
    person: PublicUser,
    cursor: string | null = null,
    isCurrent: () => boolean = () => true,
  ) => {
    setPosts((current) => ({ ...current, loading: true, error: null }))
    try {
      const page = await api.getUserPosts(person.id, cursor)
      if (!isCurrent()) return
      setPosts((current) => ({
        items: cursor ? mergeUnique(current.items, page.data) : page.data,
        cursor: page.pagination.nextCursor,
        hasMore: page.pagination.hasMore,
        loading: false,
        error: null,
      }))
    } catch (reason) {
      if (isCurrent()) {
        setPosts((current) => ({ ...current, loading: false, error: messageOf(reason, 'Could not load this profile’s posts.') }))
      }
    }
  }, [])

  useEffect(() => {
    let active = true
    setLoading(true)
    setError(null)
    setProfile(null)
    setPosts(emptyStream())
    setFollowState(null)
    api.getProfile(username)
      .then((person) => {
        if (!active) return
        setProfile(person)
        void loadProfilePosts(person, null, () => active)
      })
      .catch((reason: unknown) => {
        if (active) setError(messageOf(reason, 'Could not load this profile.'))
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => { active = false }
  }, [username, loadProfilePosts])

  async function toggleFollow() {
    if (!profile || followBusy) return
    setFollowBusy(true)
    setFollowError(null)
    try {
      const result = await api.follow(profile.id, followState !== true)
      setFollowState(result.following)
    } catch (reason) {
      setFollowError(messageOf(reason, 'Could not update follow status.'))
    } finally {
      setFollowBusy(false)
    }
  }

  if (loading) return <main className="page-content"><LoadingBlock label="Opening profile…" /></main>
  if (error || !profile) return <main className="page-content"><PageHeader title="Profile" /><ErrorBlock message={error ?? 'Profile unavailable.'} onRetry={() => window.location.reload()} /></main>

  return (
    <main className="page-content">
      <PageHeader eyebrow="A SPILL PROFILE" title="People" trailing={<span className="profile-id">#{profile.username}</span>} />
      <section className="profile-card">
        <div className="profile-cover"><span>SPILL / PROFILE</span></div>
        <div className="profile-main">
          <Avatar user={profile} size="large" />
          {!ownProfile && (
            <button className={`button ${followState ? 'button--quiet' : 'button--primary'} profile-follow`} type="button" onClick={() => void toggleFollow()} disabled={followBusy}>
              {followBusy ? 'Updating…' : followState ? 'Following' : 'Follow'}{!followState && <Icon name="plus" size={16} />}
            </button>
          )}
          <h2>{profile.displayName}</h2>
          <p className="profile-handle">@{profile.username}{profile.role === 'admin' && <span className="role-pill">Admin</span>}</p>
          <div className="profile-stats">
            <span><strong>{profile.followerCount}</strong> followers</span><span><strong>{profile.followingCount}</strong> following</span>
          </div>
          {!ownProfile && followState === null && <small className="api-limitation">Follow state isn’t included in profile reads. Use Follow to sync your state.</small>}
          {followError && <p className="inline-error" role="alert">{followError}</p>}
        </div>
      </section>
      <section className="feed-section profile-posts">
        <div className="section-heading"><div><span className="eyebrow">A TIMELINE IN THEIR OWN WORDS</span><h2>Posts</h2></div></div>
        <StreamContent stream={posts} onRetry={() => void loadProfilePosts(profile)} onMore={() => void loadProfilePosts(profile, posts.cursor)} emptyTitle="No posts yet." emptyCopy="When they share something, it will show up here." onPostChange={(post) => setPosts((current) => ({ ...current, items: current.items.map((item) => item.id === post.id ? post : item) }))} />
      </section>
    </main>
  )
}

function PostDetailPage() {
  const { postId = '' } = useParams()
  const { user } = useAuth()
  const [post, setPost] = useState<Post | null>(null)
  const [postLoading, setPostLoading] = useState(true)
  const [postError, setPostError] = useState<string | null>(null)
  const [comments, setComments] = useState<StreamState<Comment>>(emptyStream())
  const [commentBody, setCommentBody] = useState('')
  const [commentBusy, setCommentBusy] = useState(false)
  const [commentError, setCommentError] = useState<string | null>(null)

  const loadComments = useCallback(async (
    cursor: string | null = null,
    isCurrent: () => boolean = () => true,
  ) => {
    setComments((current) => ({ ...current, loading: true, error: null }))
    try {
      const page = await api.getComments(postId, cursor)
      if (!isCurrent()) return
      setComments((current) => ({
        items: cursor ? mergeUnique(current.items, page.data) : page.data,
        cursor: page.pagination.nextCursor,
        hasMore: page.pagination.hasMore,
        loading: false,
        error: null,
      }))
    } catch (reason) {
      if (isCurrent()) {
        setComments((current) => ({ ...current, loading: false, error: messageOf(reason, 'Could not load comments.') }))
      }
    }
  }, [postId])

  useEffect(() => {
    let active = true
    setPostLoading(true)
    setPostError(null)
    setPost(null)
    setComments(emptyStream())
    api.getPost(postId)
      .then((result) => { if (active) setPost(result) })
      .catch((reason: unknown) => { if (active) setPostError(messageOf(reason, 'Could not load this post.')) })
      .finally(() => { if (active) setPostLoading(false) })
    void loadComments(null, () => active)
    return () => { active = false }
  }, [postId, loadComments])

  async function addComment(event: FormEvent) {
    event.preventDefault()
    if (!commentBody.trim()) {
      setCommentError('Write a comment before sending.')
      return
    }
    setCommentBusy(true)
    setCommentError(null)
    try {
      const created = await api.createComment(postId, commentBody.trim())
      setComments((current) => ({ ...current, items: mergeUnique([created], current.items) }))
      setPost((current) => current ? { ...current, commentCount: current.commentCount + 1 } : current)
      setCommentBody('')
    } catch (reason) {
      setCommentError(messageOf(reason, 'Could not add your comment.'))
    } finally {
      setCommentBusy(false)
    }
  }

  async function removeComment(commentId: string) {
    try {
      await api.deleteComment(commentId)
      setComments((current) => ({ ...current, items: current.items.filter((item) => item.id !== commentId) }))
      setPost((current) => current ? { ...current, commentCount: Math.max(0, current.commentCount - 1) } : current)
    } catch (reason) {
      setCommentError(messageOf(reason, 'Could not delete the comment.'))
    }
  }

  return (
    <main className="page-content">
      <PageHeader eyebrow="THE CONVERSATION" title="Post details" trailing={<Link to="/" className="back-link"><Icon name="arrow-left" size={16} /> Back home</Link>} />
      {postLoading && <LoadingBlock label="Opening the conversation…" />}
      {!postLoading && postError && <ErrorBlock message={postError} onRetry={() => window.location.reload()} />}
      {post && <PostCard post={post} onChange={setPost} />}

      {post && (
        <section className="comments-section">
          <div className="section-heading"><div><span className="eyebrow">ADD YOUR VOICE</span><h2>Comments <span className="count-badge">{post.commentCount}</span></h2></div></div>
          <form className="comment-composer" onSubmit={(event) => void addComment(event)}>
            <Avatar user={user} size="small" />
            <input value={commentBody} onChange={(event) => setCommentBody(event.target.value)} placeholder="Leave a thoughtful comment…" aria-label="Comment text"  />
            <button className="send-button" type="submit" disabled={commentBusy || !commentBody.trim()} aria-label="Send comment"><Icon name="send" size={18} /></button>
          </form>
          {commentError && <p className="inline-error" role="alert">{commentError}</p>}
          {comments.loading && !comments.items.length && <LoadingBlock label="Loading comments…" />}
          {comments.error && <ErrorBlock message={comments.error} onRetry={() => void loadComments()} compact={!comments.items.length} />}
          {!comments.loading && !comments.error && !comments.items.length && <EmptyBlock title="Start the conversation." copy="No comments here yet. Leave the first one." compact />}
          <div className="comment-list">
            {comments.items.map((comment) => (
              <CommentCard key={comment.id} comment={comment} canDelete={comment.authorId === user?.id || user?.role === 'admin'} onDelete={() => void removeComment(comment.id)} />
            ))}
          </div>
          {comments.hasMore && <button className="button button--quiet load-more" type="button" disabled={comments.loading} onClick={() => void loadComments(comments.cursor)}>{comments.loading ? 'Loading…' : 'More comments'}</button>}
        </section>
      )}
    </main>
  )
}

function CommentCard({ comment, canDelete, onDelete }: { comment: Comment; canDelete: boolean; onDelete: () => void }) {
  return (
    <article className="comment-card">
      <Avatar user={comment.author} size="small" />
      <div className="comment-content">
        <div className="comment-author"><strong>{comment.author?.displayName ?? 'Spill member'}</strong><span>@{comment.author?.username ?? 'unknown'}</span><time>{new Date(comment.createdAt).toLocaleDateString()}</time></div>
        <p>{comment.body}</p>
      </div>
      {canDelete && <button className="text-button comment-delete" type="button" onClick={onDelete}>Delete</button>}
    </article>
  )
}

function LoadingBlock({ label }: { label: string }) {
  return <div className="state-card state-card--loading"><span className="spinner" /><span>{label}</span></div>
}

function EmptyBlock({ title, copy, compact = false }: { title: string; copy: string; compact?: boolean }) {
  return (
    <div className={`state-card state-card--empty ${compact ? 'state-card--compact' : ''}`}>
      <span className="empty-mark"><Icon name="spark" size={18} /></span>
      <strong>{title}</strong><p>{copy}</p>
    </div>
  )
}

function ErrorBlock({ message, onRetry, compact = false }: { message: string; onRetry: () => void; compact?: boolean }) {
  return (
    <div className={`state-card state-card--error ${compact ? 'state-card--compact' : ''}`} role="alert">
      <strong>That didn’t load.</strong><p>{message}</p>
      <button type="button" className="text-button" onClick={onRetry}>Try again</button>
    </div>
  )
}

function NotFoundPage() {
  return <main className="page-content"><PageHeader eyebrow="404 / LOST IN THOUGHT" title="That page spilled." /><EmptyBlock title="We couldn't find that page." copy="It may have moved, or the link may be incomplete." /><Link className="button button--primary" to="/">Back to home</Link></main>
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <AppFrame />
      </AuthProvider>
    </BrowserRouter>
  )
}
