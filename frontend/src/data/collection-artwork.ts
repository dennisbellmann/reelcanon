import { publicPath } from "./public-path";

export interface CollectionArtwork {
  url: string;
  titleEmbedded: boolean;
}

export function collectionArtwork(canon: {
  posterUrl?: string;
  posterTitleEmbedded?: boolean;
}): CollectionArtwork {
  return {
    url: publicPath(canon.posterUrl || "/artwork/collection-03-filmwelten.png"),
    titleEmbedded: canon.posterTitleEmbedded ?? false,
  };
}
