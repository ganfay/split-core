export type Session = {
  uuid: string;
  status: "pending" | "authenticated" | string;
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

export type User = {
  id: number;
  tg_id?: number;
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
