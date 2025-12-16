export interface TargetInfo {
  component_id: string;
  job: string;
  url: string;
  health: string;
  labels: Record<string, string>;
  discovered_labels?: Record<string, string>;
  last_scrape: string;
  last_scrape_duration?: string;
  last_error?: string;
}

export interface TargetsResponse {
  status: string;
  data: TargetInfo[];
}
