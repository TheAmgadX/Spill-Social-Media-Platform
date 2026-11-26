import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../services/api'
import type { Post } from '../types'
import { Avatar } from './Avatar'
import { Icon } from './Icon'

function timeAgo(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const minutes = Math.max(0, Math.floor((Date.now() - date.getTime()) / 60_000))
  if (minutes < 1) return 'now'
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d`
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function PostCard({
  post: initialPost,
  onChange,
}: {
  post: Post
  onChange: (post: Post) => void
}) {
  const post = initialPost
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [shareOpen, setShareOpen] = useState(false)
  const [quote, setQuote] = useState('')

  function update(next: Post) {
    onChange(next)
  }

  async function toggleLike() {
    if (busy) return
    setBusy(true)
    setError(null)
    try {
      const wasLiked = Boolean(post.liked)
      const result = await api.likePost(post.id, !wasLiked)
      update({
        ...post,
        liked: result.liked,
        likeCount: Math.max(0, post.likeCount + (result.liked === wasLiked ? 0 : result.liked ? 1 : -1)),
      })
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not update the like.')
    } finally {
      setBusy(false)
    }
  }

  async function share() {
    setBusy(true)
    setError(null)
    try {
      await api.sharePost(post.id, quote.trim())
      update({ ...post, shareCount: post.shareCount + 1 })
      setQuote('')
      setShareOpen(false)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not share this post.')
    } finally {
      setBusy(false)
    }
  }

  const author = post.author
  const authorSummary = (
    <>
      <Avatar user={author} />
      <span>
        <strong>{author?.displayName ?? 'Spill member'}</strong>
        <small>{author ? `@${author.username}` : `Member ${post.authorId.slice(0, 8)}`} <span aria-hidden="true">·</span> {timeAgo(post.createdAt)}</small>
      </span>
    </>
  )

  return (
    <article className="post-card">
      <header className="post-head">
        {author ? (
          <Link className="author-link" to={`/u/${encodeURIComponent(author.username)}`} aria-label={`View ${author.displayName} profile`}>
            {authorSummary}
          </Link>
        ) : <div className="author-link">{authorSummary}</div>}
        {author?.role === 'admin' && <span className="role-pill">Admin</span>}
      </header>

      <p className="post-body">{post.body}</p>
      {!!post.assets?.length && (
        <div className={`post-images ${post.assets.length > 1 ? 'post-images--grid' : ''}`}>
          {post.assets.map((asset) => (
            <a key={asset.id} href={asset.url} target="_blank" rel="noreferrer" className="post-image-link">
              <img src={asset.url} alt="Attached to post" loading="lazy" />
            </a>
          ))}
        </div>
      )}

      <div className="post-stats" aria-label="Post engagement">
        <span>{post.likeCount} likes</span>
        <Link to={`/p/${encodeURIComponent(post.id)}`}>{post.commentCount} comments</Link>
        <span>{post.shareCount} shares</span>
      </div>

      <div className="post-actions">
        <button
          type="button"
          className={`post-action ${post.liked ? 'post-action--liked' : ''}`}
          onClick={() => void toggleLike()}
          disabled={busy}
          aria-pressed={Boolean(post.liked)}
        >
          <Icon name="heart" size={19} />
          <span>{post.liked ? 'Liked' : 'Like'}</span>
        </button>
        <Link className="post-action" to={`/p/${encodeURIComponent(post.id)}`}>
          <Icon name="message" size={19} />
          <span>Comment</span>
        </Link>
        <button type="button" className="post-action" onClick={() => setShareOpen(true)}>
          <Icon name="share" size={18} />
          <span>Share</span>
        </button>
      </div>

      {error && <p className="inline-error" role="alert">{error}</p>}

      {shareOpen && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) setShareOpen(false)
        }}>
          <section className="dialog-card" role="dialog" aria-modal="true" aria-labelledby={`share-title-${post.id}`}>
            <div className="dialog-head">
              <h2 id={`share-title-${post.id}`}>Share this post</h2>
              <button type="button" className="icon-button" aria-label="Close dialog" onClick={() => setShareOpen(false)}>
                <Icon name="close" />
              </button>
            </div>
            <p className="muted-copy">Add a note, or share it as is.</p>
            <textarea
              value={quote}
              onChange={(event) => setQuote(event.target.value)}
              placeholder="Write a note…"
              rows={4}
              maxLength={500}
              aria-label="Optional share note"
            />
            <div className="dialog-actions">
              <button type="button" className="button button--quiet" onClick={() => setShareOpen(false)}>Cancel</button>
              <button type="button" className="button button--primary" onClick={() => void share()} disabled={busy}>
                {busy ? 'Sharing…' : 'Share post'}
              </button>
            </div>
          </section>
        </div>
      )}
    </article>
  )
}
