import type { ISkillAdapter } from '@brevio/shared';

import adapter_aerobase_skill from './aerobase-skill/index.js';
import adapter_alter_actions from './alter-actions/index.js';
import adapter_apple_contacts from './apple-contacts/index.js';
import adapter_apple_mail from './apple-mail/index.js';
import adapter_apple_mail_search from './apple-mail-search/index.js';
import adapter_apple_media from './apple-media/index.js';
import adapter_apple_music from './apple-music/index.js';
import adapter_apple_notes from './apple-notes/index.js';
import adapter_apple_notes_skill from './apple-notes-skill/index.js';
import adapter_apple_photos from './apple-photos/index.js';
import adapter_apple_remind_me from './apple-remind-me/index.js';
import adapter_asana from './asana/index.js';
import adapter_asr from './asr/index.js';
import adapter_autoresponder from './autoresponder/index.js';
import adapter_aviationstack_flight_tracker from './aviationstack-flight-tracker/index.js';
import adapter_bear_notes from './bear-notes/index.js';
import adapter_better_notion from './better-notion/index.js';
import adapter_bill_pay_p2p from './bill-pay-p2p/index.js';
import adapter_bird from './bird/index.js';
import adapter_bluesky from './bluesky/index.js';
import adapter_brave_search from './brave-search/index.js';
import adapter_buy_anything from './buy-anything/index.js';
import adapter_calctl from './calctl/index.js';
import adapter_camsnap from './camsnap/index.js';
import adapter_card_optimizer from './card-optimizer/index.js';
import adapter_chromecast from './chromecast/index.js';
import adapter_clawd_coach from './clawd-coach/index.js';
import adapter_clawringhouse from './clawringhouse/index.js';
import adapter_clickup_mcp from './clickup-mcp/index.js';
import adapter_coloring_page from './coloring-page/index.js';
import adapter_content_advisory from './content-advisory/index.js';
import adapter_contract_reviewer from './contract-reviewer/index.js';
import adapter_copilot_money from './copilot-money/index.js';
import adapter_craft from './craft/index.js';
import adapter_daily_rhythm from './daily-rhythm/index.js';
import adapter_de_ai_ify from './de-ai-ify/index.js';
import adapter_dexcom from './dexcom/index.js';
import adapter_doing_tasks from './doing-tasks/index.js';
import adapter_exa from './exa/index.js';
import adapter_excalidraw_flowchart from './excalidraw-flowchart/index.js';
import adapter_expense_tracker_pro from './expense-tracker-pro/index.js';
import adapter_fal_ai from './fal-ai/index.js';
import adapter_figma from './figma/index.js';
import adapter_financial_market_analysis from './financial-market-analysis/index.js';
import adapter_firecrawl_search from './firecrawl-search/index.js';
import adapter_flight_tracker from './flight-tracker/index.js';
import adapter_focus_mode from './focus-mode/index.js';
import adapter_food_delivery_ordering from './food-delivery-ordering/index.js';
import adapter_gamma from './gamma/index.js';
import adapter_gemini_deep_research from './gemini-deep-research/index.js';
import adapter_gemini_stt from './gemini-stt/index.js';
import adapter_george from './george/index.js';
import adapter_get_focus_mode from './get-focus-mode/index.js';
import adapter_gifhorse from './gifhorse/index.js';
import adapter_gkeep from './gkeep/index.js';
import adapter_google_calendar from './google-calendar/index.js';
import adapter_google_maps from './google-maps/index.js';
import adapter_google_workspace from './google-workspace/index.js';
import adapter_goplaces from './goplaces/index.js';
import adapter_granola from './granola/index.js';
import adapter_grocery_list from './grocery-list/index.js';
import adapter_healthkit_sync from './healthkit-sync/index.js';
import adapter_healthkit_sync_apple from './healthkit-sync-apple/index.js';
import adapter_home_assistant from './home-assistant/index.js';
import adapter_hotel_vacation_booking from './hotel-vacation-booking/index.js';
import adapter_ibkr_trading from './ibkr-trading/index.js';
import adapter_icloud_findmy from './icloud-findmy/index.js';
import adapter_imap_email from './imap-email/index.js';
import adapter_jira from './jira/index.js';
import adapter_journal_to_post from './journal-to-post/index.js';
import adapter_just_fucking_cancel from './just-fucking-cancel/index.js';
import adapter_kagi_search from './kagi-search/index.js';
import adapter_kids_family_management from './kids-family-management/index.js';
import adapter_krea_api from './krea-api/index.js';
import adapter_last30days from './last30days/index.js';
import adapter_lastfm from './lastfm/index.js';
import adapter_linear from './linear/index.js';
import adapter_literature_review from './literature-review/index.js';
import adapter_local_places from './local-places/index.js';
import adapter_local_service_booking from './local-service-booking/index.js';
import adapter_marketplace from './marketplace/index.js';
import adapter_meal_planner from './meal-planner/index.js';
import adapter_meeting_autopilot from './meeting-autopilot/index.js';
import adapter_mole_mac_cleanup from './mole-mac-cleanup/index.js';
import adapter_monarch_money from './monarch-money/index.js';
import adapter_morning_manifesto from './morning-manifesto/index.js';
import adapter_news_aggregator from './news-aggregator/index.js';
import adapter_notion from './notion/index.js';
import adapter_obsidian from './obsidian/index.js';
import adapter_omnifocus from './omnifocus/index.js';
import adapter_openai_tts from './openai-tts/index.js';
import adapter_outlook from './outlook/index.js';
import adapter_overseerr from './overseerr/index.js';
import adapter_parcel_package_tracking from './parcel-package-tracking/index.js';
import adapter_pdf_tools from './pdf-tools/index.js';
import adapter_perplexity from './perplexity/index.js';
import adapter_personal_shopper from './personal-shopper/index.js';
import adapter_pet_care from './pet-care/index.js';
import adapter_pharmacy_prescription from './pharmacy-prescription/index.js';
import adapter_plaid from './plaid/index.js';
import adapter_plan_my_day from './plan-my-day/index.js';
import adapter_plex from './plex/index.js';
import adapter_pocket_casts from './pocket-casts/index.js';
import adapter_pollinations from './pollinations/index.js';
import adapter_post_at from './post-at/index.js';
import adapter_proactive_research from './proactive-research/index.js';
import adapter_pros_cons from './pros-cons/index.js';
import adapter_radarr from './radarr/index.js';
import adapter_react_email_skills from './react-email-skills/index.js';
import adapter_recipe_to_list from './recipe-to-list/index.js';
import adapter_reddit from './reddit/index.js';
import adapter_reflect from './reflect/index.js';
import adapter_refund_radar from './refund-radar/index.js';
import adapter_relationship_skills from './relationship-skills/index.js';
import adapter_restaurant_reservations from './restaurant-reservations/index.js';
import adapter_resume_builder from './resume-builder/index.js';
import adapter_ride_hailing from './ride-hailing/index.js';
import adapter_roku from './roku/index.js';
import adapter_sag from './sag/index.js';
import adapter_samsung_smart_tv from './samsung-smart-tv/index.js';
import adapter_second_brain from './second-brain/index.js';
import adapter_self_improvement from './self-improvement/index.js';
import adapter_serpapi from './serpapi/index.js';
import adapter_shopping_expert from './shopping-expert/index.js';
import adapter_shortcuts_generator from './shortcuts-generator/index.js';
import adapter_slack from './slack/index.js';
import adapter_sleep_calculator from './sleep-calculator/index.js';
import adapter_smart_expense_tracker from './smart-expense-tracker/index.js';
import adapter_smtp_send from './smtp-send/index.js';
import adapter_sonarr from './sonarr/index.js';
import adapter_sonoscli from './sonoscli/index.js';
import adapter_sports_ticker from './sports-ticker/index.js';
import adapter_spotify from './spotify/index.js';
import adapter_spotify_history from './spotify-history/index.js';
import adapter_spotify_player from './spotify-player/index.js';
import adapter_spotify_web_api from './spotify-web-api/index.js';
import adapter_spots from './spots/index.js';
import adapter_streaming_recommendations from './streaming-recommendations/index.js';
import adapter_swissweather from './swissweather/index.js';
import adapter_tavily from './tavily/index.js';
import adapter_tax_professional from './tax-professional/index.js';
import adapter_things_mac from './things-mac/index.js';
import adapter_thinking_partner from './thinking-partner/index.js';
import adapter_ticktick from './ticktick/index.js';
import adapter_tmdb from './tmdb/index.js';
import adapter_todo from './todo/index.js';
import adapter_todoist from './todoist/index.js';
import adapter_track17 from './track17/index.js';
import adapter_trakt from './trakt/index.js';
import adapter_trello from './trello/index.js';
import adapter_veo from './veo/index.js';
import adapter_video_frames from './video-frames/index.js';
import adapter_video_transcript_downloader from './video-transcript-downloader/index.js';
import adapter_vocal_chat from './vocal-chat/index.js';
import adapter_voice_wake_say from './voice-wake-say/index.js';
import adapter_watch_my_money from './watch-my-money/index.js';
import adapter_whatsapp_styling_guide from './whatsapp-styling-guide/index.js';
import adapter_withings_health from './withings-health/index.js';
import adapter_yahoo_finance from './yahoo-finance/index.js';
import adapter_ynab from './ynab/index.js';
import adapter_youtube_api from './youtube-api/index.js';
import adapter_youtube_summarizer from './youtube-summarizer/index.js';
import adapter_ytmusic from './ytmusic/index.js';

export const SkillRegistry: Record<string, ISkillAdapter> = {
  'aerobase-skill': adapter_aerobase_skill,
  'alter-actions': adapter_alter_actions,
  'apple-contacts': adapter_apple_contacts,
  'apple-mail': adapter_apple_mail,
  'apple-mail-search': adapter_apple_mail_search,
  'apple-media': adapter_apple_media,
  'apple-music': adapter_apple_music,
  'apple-notes': adapter_apple_notes,
  'apple-notes-skill': adapter_apple_notes_skill,
  'apple-photos': adapter_apple_photos,
  'apple-remind-me': adapter_apple_remind_me,
  'asana': adapter_asana,
  'asr': adapter_asr,
  'autoresponder': adapter_autoresponder,
  'aviationstack-flight-tracker': adapter_aviationstack_flight_tracker,
  'bear-notes': adapter_bear_notes,
  'better-notion': adapter_better_notion,
  'bill-pay-p2p': adapter_bill_pay_p2p,
  'bird': adapter_bird,
  'bluesky': adapter_bluesky,
  'brave-search': adapter_brave_search,
  'buy-anything': adapter_buy_anything,
  'calctl': adapter_calctl,
  'camsnap': adapter_camsnap,
  'card-optimizer': adapter_card_optimizer,
  'chromecast': adapter_chromecast,
  'clawd-coach': adapter_clawd_coach,
  'clawringhouse': adapter_clawringhouse,
  'clickup-mcp': adapter_clickup_mcp,
  'coloring-page': adapter_coloring_page,
  'content-advisory': adapter_content_advisory,
  'contract-reviewer': adapter_contract_reviewer,
  'copilot-money': adapter_copilot_money,
  'craft': adapter_craft,
  'daily-rhythm': adapter_daily_rhythm,
  'de-ai-ify': adapter_de_ai_ify,
  'dexcom': adapter_dexcom,
  'doing-tasks': adapter_doing_tasks,
  'exa': adapter_exa,
  'excalidraw-flowchart': adapter_excalidraw_flowchart,
  'expense-tracker-pro': adapter_expense_tracker_pro,
  'fal-ai': adapter_fal_ai,
  'figma': adapter_figma,
  'financial-market-analysis': adapter_financial_market_analysis,
  'firecrawl-search': adapter_firecrawl_search,
  'flight-tracker': adapter_flight_tracker,
  'focus-mode': adapter_focus_mode,
  'food-delivery-ordering': adapter_food_delivery_ordering,
  'gamma': adapter_gamma,
  'gemini-deep-research': adapter_gemini_deep_research,
  'gemini-stt': adapter_gemini_stt,
  'george': adapter_george,
  'get-focus-mode': adapter_get_focus_mode,
  'gifhorse': adapter_gifhorse,
  'gkeep': adapter_gkeep,
  'google-calendar': adapter_google_calendar,
  'google-maps': adapter_google_maps,
  'google-workspace': adapter_google_workspace,
  'goplaces': adapter_goplaces,
  'granola': adapter_granola,
  'grocery-list': adapter_grocery_list,
  'healthkit-sync': adapter_healthkit_sync,
  'healthkit-sync-apple': adapter_healthkit_sync_apple,
  'home-assistant': adapter_home_assistant,
  'hotel-vacation-booking': adapter_hotel_vacation_booking,
  'ibkr-trading': adapter_ibkr_trading,
  'icloud-findmy': adapter_icloud_findmy,
  'imap-email': adapter_imap_email,
  'jira': adapter_jira,
  'journal-to-post': adapter_journal_to_post,
  'just-fucking-cancel': adapter_just_fucking_cancel,
  'kagi-search': adapter_kagi_search,
  'kids-family-management': adapter_kids_family_management,
  'krea-api': adapter_krea_api,
  'last30days': adapter_last30days,
  'lastfm': adapter_lastfm,
  'linear': adapter_linear,
  'literature-review': adapter_literature_review,
  'local-places': adapter_local_places,
  'local-service-booking': adapter_local_service_booking,
  'marketplace': adapter_marketplace,
  'meal-planner': adapter_meal_planner,
  'meeting-autopilot': adapter_meeting_autopilot,
  'mole-mac-cleanup': adapter_mole_mac_cleanup,
  'monarch-money': adapter_monarch_money,
  'morning-manifesto': adapter_morning_manifesto,
  'news-aggregator': adapter_news_aggregator,
  'notion': adapter_notion,
  'obsidian': adapter_obsidian,
  'omnifocus': adapter_omnifocus,
  'openai-tts': adapter_openai_tts,
  'outlook': adapter_outlook,
  'overseerr': adapter_overseerr,
  'parcel-package-tracking': adapter_parcel_package_tracking,
  'pdf-tools': adapter_pdf_tools,
  'perplexity': adapter_perplexity,
  'personal-shopper': adapter_personal_shopper,
  'pet-care': adapter_pet_care,
  'pharmacy-prescription': adapter_pharmacy_prescription,
  'plaid': adapter_plaid,
  'plan-my-day': adapter_plan_my_day,
  'plex': adapter_plex,
  'pocket-casts': adapter_pocket_casts,
  'pollinations': adapter_pollinations,
  'post-at': adapter_post_at,
  'proactive-research': adapter_proactive_research,
  'pros-cons': adapter_pros_cons,
  'radarr': adapter_radarr,
  'react-email-skills': adapter_react_email_skills,
  'recipe-to-list': adapter_recipe_to_list,
  'reddit': adapter_reddit,
  'reflect': adapter_reflect,
  'refund-radar': adapter_refund_radar,
  'relationship-skills': adapter_relationship_skills,
  'restaurant-reservations': adapter_restaurant_reservations,
  'resume-builder': adapter_resume_builder,
  'ride-hailing': adapter_ride_hailing,
  'roku': adapter_roku,
  'sag': adapter_sag,
  'samsung-smart-tv': adapter_samsung_smart_tv,
  'second-brain': adapter_second_brain,
  'self-improvement': adapter_self_improvement,
  'serpapi': adapter_serpapi,
  'shopping-expert': adapter_shopping_expert,
  'shortcuts-generator': adapter_shortcuts_generator,
  'slack': adapter_slack,
  'sleep-calculator': adapter_sleep_calculator,
  'smart-expense-tracker': adapter_smart_expense_tracker,
  'smtp-send': adapter_smtp_send,
  'sonarr': adapter_sonarr,
  'sonoscli': adapter_sonoscli,
  'sports-ticker': adapter_sports_ticker,
  'spotify': adapter_spotify,
  'spotify-history': adapter_spotify_history,
  'spotify-player': adapter_spotify_player,
  'spotify-web-api': adapter_spotify_web_api,
  'spots': adapter_spots,
  'streaming-recommendations': adapter_streaming_recommendations,
  'swissweather': adapter_swissweather,
  'tavily': adapter_tavily,
  'tax-professional': adapter_tax_professional,
  'things-mac': adapter_things_mac,
  'thinking-partner': adapter_thinking_partner,
  'ticktick': adapter_ticktick,
  'tmdb': adapter_tmdb,
  'todo': adapter_todo,
  'todoist': adapter_todoist,
  'track17': adapter_track17,
  'trakt': adapter_trakt,
  'trello': adapter_trello,
  'veo': adapter_veo,
  'video-frames': adapter_video_frames,
  'video-transcript-downloader': adapter_video_transcript_downloader,
  'vocal-chat': adapter_vocal_chat,
  'voice-wake-say': adapter_voice_wake_say,
  'watch-my-money': adapter_watch_my_money,
  'whatsapp-styling-guide': adapter_whatsapp_styling_guide,
  'withings-health': adapter_withings_health,
  'yahoo-finance': adapter_yahoo_finance,
  'ynab': adapter_ynab,
  'youtube-api': adapter_youtube_api,
  'youtube-summarizer': adapter_youtube_summarizer,
  'ytmusic': adapter_ytmusic,
};

export function getSkillAdapter(skillId: string): ISkillAdapter | null {
  return SkillRegistry[skillId] ?? null;
}
