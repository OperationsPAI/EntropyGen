import { type HTMLAttributes, type ReactNode } from 'react'
import styles from './Card.module.css'

interface CardProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  title?: ReactNode
}

export default function Card({ title, children, className, ...rest }: CardProps) {
  return (
    <div className={`${styles.card} ${className ?? ''}`} {...rest}>
      {title && <div className={styles.title}>{title}</div>}
      {children}
    </div>
  )
}
