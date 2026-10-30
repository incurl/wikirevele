import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/**
 * Joins the class names of a component, and lets the later ones win.
 *
 * This is the helper shadcn/ui uses everywhere: a component sets its
 * own classes and the caller passes the overrides, and the overrides
 * are the ones that survive.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
