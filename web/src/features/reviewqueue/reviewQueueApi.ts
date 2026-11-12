import { useQuery } from '@tanstack/react-query'
import { listReviewsOptions } from '@/api/generated/@tanstack/react-query.gen.ts'
import { freshFor } from '@/queryClient.ts'

// useReviewQueue reads the pull requests on the forge that wait on your review,
// the longest-waiting first.
export function useReviewQueue() {
  // The queue is a search of the forge across repositories, under its rate
  // limits, and nothing pushes a newer one: past freshFor, opening the section
  // reads it again, and Refresh reads it at once. Nothing polls — a queue
  // nobody is looking at is not worth a request.
  return useQuery({ ...listReviewsOptions(), staleTime: freshFor })
}
