export type SessionStatus = "pending" | "authenticated" | (string & {});

export type Session = {
  uuid: string;
  status: SessionStatus;
  access_token?: string;
  refresh_token?: string;
  firstname?: string;
  username?: string;
  iid?: number;
  tg_id?: number;
};

export type SessionCheckRequest = {
  uuid: string;
};

export type Tokens = {
  access_token: string;
};

export type Fund = {
  id: number;
  name: string;
  author_id: number;
  invite_code: string;
  created_at: string;
};

export type PaginationRequest = {
  limit: number;
  offset: number;
};

export type FundRequest = {
  id: number;
  invite_code?: string;
};

export type JoinFundRequest = {
  invite_code: string;
};

export type FundPaginationRequest = {
  fund_id: number;
  limit: number;
  offset: number;
};

export type VirtualUserRequest = {
  fund_id: number;
  first_name: string;
};

export type RemoveUserRequest = {
  fund_id: number;
  user_id: number;
};

export type AddExpenseRequest = {
  fund_id: number;
  description: string;
  cost: number;
};

export type User = {
  id: number;
  tg_id?: number | null;
  username: string;
  first_name: string;
  is_virtual: boolean;
  created_at?: string;
};

export type Debt = {
  from_id: number;
  to_id: number;
  amount: number;
};

export type SettlementResponse = {
  total_amount: number;
  average: number;
  debts: Debt[] | null;
};

export type Settlement = {
  total_amount: number;
  average: number;
  debts: Debt[];
};

export type Purchase = {
  id: number;
  fund_id: number;
  payer: User;
  amount: number;
  description: string;
  created_at: string;
};

export type ResponseError = {
  error: string;
};
