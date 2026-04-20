type PinIconProps = { className?: string }

export function PinIcon({ className }: PinIconProps) {
  return (
    <svg width="15" height="15" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className={className} aria-hidden="true">
      <path d="M9.75 1.41667L6.41667 4.75L3.08333 6L1.83333 7.25L7.66667 13.0833L8.91667 11.8333L10.1667 8.5L13.5 5.16667M4.75 10.1667L1 13.9167M9.33333 1L13.9167 5.58333" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  )
}

export function UnpinIcon({ className }: PinIconProps) {
  return (
    <svg width="15" height="15" viewBox="0 0 15 15" fill="none" xmlns="http://www.w3.org/2000/svg" className={className} aria-hidden="true">
      <path d="M9.75 1.41667L6.41667 4.75L3.08333 6L1.83333 7.25L7.66667 13.0833L8.91667 11.8333L10.1667 8.5L13.5 5.16667" fill="currentColor"/>
      <path d="M4.75 10.1667L1 13.9167Z" fill="currentColor"/>
      <path d="M9.33333 1L13.9167 5.58333Z" fill="currentColor"/>
      <path d="M9.75 1.41667L6.41667 4.75L3.08333 6L1.83333 7.25L7.66667 13.0833L8.91667 11.8333L10.1667 8.5L13.5 5.16667M4.75 10.1667L1 13.9167M9.33333 1L13.9167 5.58333" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"/>
    </svg>
  )
}
