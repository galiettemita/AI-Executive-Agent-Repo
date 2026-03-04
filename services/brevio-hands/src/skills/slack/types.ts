export interface SlackInput {
  action: 'list_channels' | 'post_message' | 'add_reaction';
  channel_id?: string;
  text?: string;
  message_ts?: string;
  emoji?: string;
}

export interface SlackChannel {
  id: string;
  name: string;
}

export interface SlackPost {
  channel_id: string;
  message_ts: string;
  text: string;
}

export interface SlackOutput {
  provider: 'slack';
  action: SlackInput['action'];
  channels?: SlackChannel[];
  post?: SlackPost;
  reacted?: boolean;
}
