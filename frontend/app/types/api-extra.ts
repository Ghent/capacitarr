/**
 * Client-only types that are not REST resources in the published OpenAPI spec.
 * Resource types live in generated/openapi.ts and are re-exported from api.ts.
 */

export interface ApiError {
  data?: {
    error?: string;
  };
  message?: string;
}

export interface SelectedDetailItem {
  mediaName: string;
  mediaType: string;
  _score: number;
  scoreDetails: string;
  sizeBytes: number;
  action: string;
  createdAt: string;
  collectionGroup?: string;
}
