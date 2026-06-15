import type { BranchInfo } from './api';

export type BranchSelectOption = {
  value: string;
  label: string;
  description: string;
  searchText: string;
  isRemote: boolean;
  isCurrent: boolean;
};

function sortBranches(branches: BranchInfo[]) {
  return [...branches].sort((left, right) => {
    if (left.isCurrent !== right.isCurrent) {
      return left.isCurrent ? -1 : 1;
    }
    if (left.isRemote !== right.isRemote) {
      return left.isRemote ? -1 : 1;
    }
    return left.name.localeCompare(right.name);
  });
}

export function buildBranchOptions(branches: BranchInfo[]): BranchSelectOption[] {
  return sortBranches(branches).map((branch) => ({
    value: branch.name,
    label: branch.name,
    description: branch.isRemote ? '远程分支' : '本地分支',
    searchText: `${branch.name} ${branch.isRemote ? 'remote 远程 origin' : 'local 本地'} ${branch.isCurrent ? 'current 当前' : ''}`,
    isRemote: branch.isRemote,
    isCurrent: branch.isCurrent,
  }));
}

export function buildBranchMentionItems(branches: BranchInfo[], query: string) {
  const needle = query.trim().toLowerCase();

  return buildBranchOptions(branches)
    .filter((branch) => {
      if (!needle) {
        return true;
      }

      return [branch.label, branch.description, branch.searchText]
        .join(' ')
        .toLowerCase()
        .includes(needle);
    })
    .map((branch) => ({
      id: `resource-branch-${branch.value}`,
      type: 'branch' as const,
      label: branch.label,
      value: branch.value,
      description: branch.isCurrent
        ? `${branch.description} · 当前分支`
        : branch.description,
      category: 'branch',
    }));
}
