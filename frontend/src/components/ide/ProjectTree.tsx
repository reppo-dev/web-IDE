import React, { useMemo, useState } from 'react';
import { FileCode, Folder, ChevronRight, ChevronDown, Trash2 } from 'lucide-react';
import { CodeFile } from '../../types';

interface TreeNode {
  name: string;
  path: string;
  isDir: boolean;
  children: TreeNode[];
}

/**
 * buildTree turns flat paths like "backend/api/router.go" into a nested tree.
 * Folders sort before files, and both alphabetically, which is what a file
 * explorer is expected to do. Flat lists of full paths are unreadable once a
 * project has more than a handful of files.
 */
export function buildTree(paths: string[]): TreeNode {
  const root: TreeNode = { name: '', path: '', isDir: true, children: [] };

  for (const fullPath of paths) {
    const segments = fullPath.split('/').filter(Boolean);
    let current = root;

    segments.forEach((segment, index) => {
      const isLeaf = index === segments.length - 1;
      const pathSoFar = segments.slice(0, index + 1).join('/');

      let child = current.children.find((c) => c.name === segment && c.isDir === !isLeaf);
      if (!child) {
        child = { name: segment, path: pathSoFar, isDir: !isLeaf, children: [] };
        current.children.push(child);
      }
      current = child;
    });
  }

  const sortRecursive = (node: TreeNode) => {
    node.children.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' });
    });
    node.children.forEach(sortRecursive);
  };
  sortRecursive(root);

  return root;
}

function collectDirPaths(node: TreeNode, out: Set<string>) {
  if (node.isDir && node.path) out.add(node.path);
  node.children.forEach((c) => collectDirPaths(c, out));
}

interface ProjectTreeProps {
  files: CodeFile[];
  activeFile: string;
  rootName?: string;
  onSelectFile: (path: string) => void;
  onDeleteFile?: (path: string) => void;
}

/**
 * ProjectTree renders a project's files as a collapsible folder tree. Every
 * folder is open until the user closes it, so the structure of a freshly
 * opened project is visible without any clicking.
 */
export const ProjectTree: React.FC<ProjectTreeProps> = ({
  files,
  activeFile,
  rootName,
  onSelectFile,
  onDeleteFile,
}) => {
  const tree = useMemo(() => buildTree(files.map((f) => f.path)), [files]);

  const allDirs = useMemo(() => {
    const set = new Set<string>();
    collectDirPaths(tree, set);
    return set;
  }, [tree]);

  const [closedDirs, setClosedDirs] = useState<Set<string>>(new Set());

  const expanded = useMemo(() => {
    const open = new Set<string>();
    for (const dir of allDirs) {
      if (!closedDirs.has(dir)) open.add(dir);
    }
    return open;
  }, [allDirs, closedDirs]);

  const toggle = (path: string) => {
    setClosedDirs((prev) => {
      const next = new Set(prev);
      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }
      return next;
    });
  };

  const renderNode = (node: TreeNode, depth: number): React.ReactNode => {
    const paddingLeft = 6 + depth * 12;

    if (node.isDir) {
      const isOpen = expanded.has(node.path);
      return (
        <React.Fragment key={node.path}>
          <button
            onClick={() => toggle(node.path)}
            className="w-full flex items-center gap-1 py-[3px] pr-2 rounded text-[11px] text-zinc-400 hover:bg-zinc-800/60 hover:text-zinc-200 transition-colors"
            style={{ paddingLeft }}
            title={node.path}
          >
            {isOpen ? <ChevronDown size={12} className="shrink-0" /> : <ChevronRight size={12} className="shrink-0" />}
            <Folder size={12} className="text-indigo-400 shrink-0" />
            <span className="truncate font-medium">{node.name}</span>
          </button>
          {isOpen && node.children.map((child) => renderNode(child, depth + 1))}
        </React.Fragment>
      );
    }

    const isActive = activeFile === node.path;
    return (
      <div
        key={node.path}
        onClick={() => onSelectFile(node.path)}
        className={`group flex items-center justify-between pr-2 py-[3px] rounded text-[11px] cursor-pointer transition-colors ${
          isActive
            ? 'bg-zinc-800 text-zinc-100 font-medium border-l-2 border-indigo-500'
            : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200'
        }`}
        style={{ paddingLeft }}
        title={node.path}
      >
        <div className="flex items-center gap-1.5 min-w-0">
          <span className="w-[12px] shrink-0" />
          <FileCode size={12} className={isActive ? 'text-indigo-400 shrink-0' : 'text-zinc-500 shrink-0'} />
          <span className="truncate">{node.name}</span>
        </div>
        {onDeleteFile && (
          <button
            onClick={(e) => {
              e.stopPropagation();
              onDeleteFile(node.path);
            }}
            className="opacity-0 group-hover:opacity-100 p-0.5 text-zinc-500 hover:text-red-400 transition-opacity shrink-0"
            title="حذف"
          >
            <Trash2 size={10} />
          </button>
        )}
      </div>
    );
  };

  return (
    <div className="py-1">
      {rootName && (
        <div className="flex items-center gap-1 px-2 py-1 text-zinc-300 font-semibold text-[11px] border-b border-zinc-800/60 mb-1">
          <ChevronDown size={12} />
          <Folder size={12} className="text-indigo-400" />
          <span className="truncate">{rootName}</span>
        </div>
      )}
      <div className="space-y-[1px]">{tree.children.map((node) => renderNode(node, 0))}</div>
    </div>
  );
};