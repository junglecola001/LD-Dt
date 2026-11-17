import React from 'react';
import styles from './ui.module.css';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
}

export function Button({ variant = 'secondary', className = '', ...rest }: ButtonProps) {
  const variantClass =
    variant === 'primary'
      ? styles.btnPrimary
      : variant === 'ghost'
        ? styles.btnGhost
        : variant === 'danger'
          ? styles.btnDanger
          : styles.btnSecondary;

  return <button type="button" className={`${styles.btn} ${variantClass} ${className}`} {...rest} />;
}

interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  label: string;
  danger?: boolean;
}

export function IconButton({ label, danger = false, className = '', children, ...rest }: IconButtonProps) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={`${styles.iconBtn} ${danger ? styles.iconBtnDanger : ''} ${className}`}
      {...rest}
    >
      {children}
    </button>
  );
}

export function EmptyState({
  lead,
  actionLabel,
  onAction,
}: {
  lead: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <div className={styles.emptyState}>
      <p className={styles.emptyLead}>{lead}</p>
      {actionLabel && onAction && (
        <Button variant="primary" onClick={onAction}>
          {actionLabel}
        </Button>
      )}
    </div>
  );
}
