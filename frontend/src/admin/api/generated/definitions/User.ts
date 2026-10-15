/* eslint-disable prettier/prettier */
// 本文件由 `cd frontend && bash scripts/api-gen.sh` 自动生成，请勿手工修改。
import type { Definition } from './Definition';
import type { DeletedAt } from './DeletedAt';
import type { Dept } from './Dept';
import type { Post } from './Post';
import type { Role } from './Role';

export type User = Definition & {
	active?: boolean;
	avatar?: string;
	created_by?: string;
	created_dt?: string;
	deleted_at?: DeletedAt;
	dept?: Dept;
	dept_id?: number;
	email?: string;
	id?: number;
	is_admin?: boolean;
	last_login?: string;
	name?: string;
	phone?: string;
	post?: Post;
	post_id?: number;
	remark?: string;
	roles?: Role[];
	updated_by?: string;
	updated_dt?: string;
	user_name?: string;
}
