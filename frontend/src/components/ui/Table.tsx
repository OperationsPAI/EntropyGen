import { type TableHTMLAttributes } from 'react'
import styles from './Table.module.css'

interface TableProps extends TableHTMLAttributes<HTMLTableElement> {}

export default function Table({ className, children, ...rest }: TableProps) {
  return (
    <table className={`${styles.table} ${className ?? ''}`} {...rest}>
      {children}
    </table>
  )
}
