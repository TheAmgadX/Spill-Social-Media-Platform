import type { PublicUser } from '../types'

export function Avatar({
  user,
  size = 'medium',
}: {
  user?: Pick<PublicUser, 'username' | 'displayName' | 'avatarUrl'> | null
  size?: 'small' | 'medium' | 'large'
}) {
  const initials = user?.displayName
    ?.split(/\s+/)
    .map((part) => part[0])
    .join('')
    .slice(0, 2)
    .toUpperCase() || user?.username?.slice(0, 2).toUpperCase() || '?'

  return (
    <span className={`avatar avatar--${size}`} aria-hidden={!user?.avatarUrl}>
      {user?.avatarUrl ? <img src={user.avatarUrl} alt="" /> : initials}
    </span>
  )
}
