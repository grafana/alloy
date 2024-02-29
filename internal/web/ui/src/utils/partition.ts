import { PartitionedBody } from '../features/component/types';
import { AttrStmt, Body, StmtType } from '../features/river-js/types';

/**
 * partitionBody groups a body by attributes and inner blocks, assigning unique
 * keys for each.
 */
export function partitionBody(body: Body, rootKey: string): PartitionedBody {
  function impl(body: Body, displayName: string[], keyPath: string[]): PartitionedBody {
    const attrs: AttrStmt[] = [];
    const inner: PartitionedBody[] = [];

    const blocksWithName: Record<string, number> = {};

    body.forEach((stmt) => {
      switch (stmt.type) {
        case StmtType.ATTR:
          attrs.push(stmt);
          break;
        case StmtType.BLOCK:
          const blockName = stmt.label ? `${stmt.name}.${stmt.label}` : stmt.name;

          // Keep track of how many blocks have this name so they can be given unique IDs.
          if (blocksWithName[blockName] === undefined) {
            blocksWithName[blockName] = 0;
          }
          const number = blocksWithName[blockName];
          blocksWithName[blockName]++;

          const key = blockName + `_${number}`;

          inner.push(impl(stmt.body, displayName.concat([blockName]), keyPath.concat([key])));
          break;
      }
    });

    return {
      displayName: displayName,
      key: keyPath,
      attrs: attrs,
      inner: inner,
    };
  }

  return impl(body, [rootKey], [rootKey]);
}
