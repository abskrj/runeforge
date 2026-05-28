import clsx from 'clsx'

interface Props {
  language: string
  className?: string
}

const colors: Record<string, string> = {
  bun: 'bg-yellow-100 text-yellow-800',
  python: 'bg-blue-100 text-blue-800',
}

export default function LanguageBadge({ language, className }: Props) {
  return (
    <span
      className={clsx(
        'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
        colors[language] ?? 'bg-gray-100 text-gray-800',
        className,
      )}
    >
      {language}
    </span>
  )
}
