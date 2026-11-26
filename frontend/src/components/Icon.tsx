import type { ReactNode, SVGProps } from 'react'

export type IconName =
  | 'home'
  | 'search'
  | 'user'
  | 'heart'
  | 'message'
  | 'share'
  | 'image'
  | 'plus'
  | 'arrow-left'
  | 'arrow-right'
  | 'close'
  | 'send'
  | 'logout'
  | 'spark'
  | 'users'

export function Icon({
  name,
  size = 20,
  ...props
}: SVGProps<SVGSVGElement> & { name: IconName; size?: number }) {
  const common = {
    width: size,
    height: size,
    viewBox: '0 0 24 24',
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.8,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
    'aria-hidden': true as const,
    ...props,
  }

  const paths: Record<IconName, ReactNode> = {
    home: <><path d="m3 10 9-7 9 7" /><path d="M5 9v11h14V9M9 20v-6h6v6" /></>,
    search: <><circle cx="10.8" cy="10.8" r="6.8" /><path d="m16 16 4.5 4.5" /></>,
    user: <><circle cx="12" cy="8" r="3.3" /><path d="M5 21c.7-3.4 3.2-5.2 7-5.2s6.3 1.8 7 5.2" /></>,
    heart: <path d="M20.5 8.7c0 5-8.5 10.1-8.5 10.1S3.5 13.7 3.5 8.7a4.2 4.2 0 0 1 7.8-2.2l.7 1 .7-1a4.2 4.2 0 0 1 7.8 2.2Z" />,
    message: <><path d="M20 11.5a7.5 7.5 0 0 1-7.5 7.5H6l-3 2v-9.5A7.5 7.5 0 0 1 10.5 4h2A7.5 7.5 0 0 1 20 11.5Z" /><path d="M8 11h8M8 14h5" /></>,
    share: <><path d="M12 15V3m0 0L7.5 7.5M12 3l4.5 4.5" /><path d="M5 12v8h14v-8" /></>,
    image: <><rect x="3" y="4" width="18" height="16" rx="2" /><circle cx="8.5" cy="9" r="1.5" /><path d="m21 15-5-5L5 20" /></>,
    plus: <><path d="M12 5v14M5 12h14" /></>,
    'arrow-left': <><path d="m14 18-6-6 6-6" /><path d="M20 12H8" /></>,
    'arrow-right': <><path d="m10 18 6-6-6-6" /><path d="M4 12h12" /></>,
    close: <><path d="m18 6-12 12M6 6l12 12" /></>,
    send: <><path d="m22 2-7 20-4-9-9-4Z" /><path d="M22 2 11 13" /></>,
    logout: <><path d="M10 17l5-5-5-5M15 12H3" /><path d="M12 3h7a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-7" /></>,
    spark: <><path d="m12 3 1.7 5.3L19 10l-5.3 1.7L12 17l-1.7-5.3L5 10l5.3-1.7L12 3Z" /><path d="m19 16 .8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8L19 16Z" /></>,
    users: <><circle cx="9" cy="8" r="3" /><path d="M3 20c.5-3 2.5-4.5 6-4.5s5.5 1.5 6 4.5M16 5.5a3 3 0 0 1 0 5.8M18 15.5c1.8.6 2.8 2.1 3 4.5" /></>,
  }

  return <svg {...common}>{paths[name]}</svg>
}
