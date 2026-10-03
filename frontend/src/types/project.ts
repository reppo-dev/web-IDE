export interface ProjectRef {
  id: string;
  name: string;
  root: string;
}

export interface ProjectFileEntry {
  path: string;
  size: number;
}

export interface ProjectTree {
  id: string;
  name: string;
  root: string;
  files: ProjectFileEntry[];
  truncated: boolean;
}

export interface ProjectFileContent {
  path: string;
  language: string;
  content: string;
  truncated: boolean;
}