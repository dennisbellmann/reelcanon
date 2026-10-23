import type { Movie } from "../api/client";
import { useTranslation } from "react-i18next";

type Motif =
  | "bird"
  | "book"
  | "candle"
  | "camera"
  | "city"
  | "crown"
  | "eye"
  | "flower"
  | "globe"
  | "hotel"
  | "house"
  | "lightning"
  | "mask"
  | "microphone"
  | "mirror"
  | "moon"
  | "pod"
  | "road"
  | "ship"
  | "spotlight"
  | "train"
  | "wave"
  | "wing";

const horrorMotifs: [RegExp, Motif][] = [
  [
    /get out|repulsion|the vanishing|the innocents|dr\. jekyll|gerald's game|the endless|10 cloverfield lane|don't look now|it follows|the night of the hunter/i,
    "mirror",
  ],
  [
    /under the shadow|his house|housebound|the cabin in the woods|the invitation|we are still here|village of the damned|cat and the canary|the cabin|house$/i,
    "house",
  ],
  [
    /the loved ones|freaks|raw|house of wax|devil's candy|the wicker man|cat people|1922/i,
    "candle",
  ],
  [/suspiria|teenage sex and death at camp miasma/i, "spotlight"],
  [
    /tigers are not afraid|resident evil|weapons|send help|green room|bone tomahawk|bone temple|leviticus|the fly/i,
    "eye",
  ],
  [/host$/i, "camera"],
  [/babadook|book|caligari/i, "book"],
  [/ringu|ring|well/i, "eye"],
  [/frankenstein|lightning/i, "lightning"],
  [/king kong|kong/i, "city"],
  [/tiger/i, "eye"],
  [
    /halloween|pumpkin|mummy|dracula|vampyr|nosferatu|wolf man|werewolf/i,
    "moon",
  ],
  [/rosemary|baby|hereditary|house|home|cabin|hokum|invitation/i, "house"],
  [/eyes without|invisible|face|mask|phantom/i, "mask"],
  [/harpoon|wailing|llorona|host|water|river/i, "wave"],
  [/body snatch|pod/i, "pod"],
  [/quiet place|birds|bird/i, "bird"],
  [/train|busan/i, "train"],
  [/witch|carrie|love witch|ginger|nina|bride/i, "flower"],
  [/peeping tom|camera|creepy|creep/i, "camera"],
  [/night|shadow|vanishing|dark|sinners|devil|demon|obsession/i, "mirror"],
  [/duel|road|revenge|attack the block/i, "road"],
  [/lighthouse|light/i, "spotlight"],
  [/the fly|fly/i, "eye"],
  [/candy|raw|wax|freaks|witch/i, "candle"],
];

const oscarMotifs: [RegExp, Motif][] = [
  [/from here to eternity|the english patient/i, "wave"],
  [/you can't take it with you/i, "house"],
  [
    /ordinary people|kramer vs\. kramer|gentleman's agreement|the lost weekend/i,
    "mirror",
  ],
  [
    /in the heat of the night|slumdog millionaire|marty|crash|the french connection|annie hall/i,
    "city",
  ],
  [/chariots of fire|going my way|dances with wolves/i, "road"],
  [/the great ziegfeld|the artist|oliver!/i, "spotlight"],
  [/wings|air|flight|argo/i, "wing"],
  [/broadway|melody|ziegfeld|chicago|artist|jazz|sing|music/i, "spotlight"],
  [/quiet on the western|war|patton|hurt locker|battle|soldier/i, "road"],
  [/cimarron|valley|out of africa|nomadland|green book/i, "wave"],
  [/hotel|grand hotel/i, "hotel"],
  [/cavalcade|around the world|80 days|journey|road/i, "globe"],
  [/mutiny|bounty|titanic|ship|water/i, "ship"],
  [/night|moonlight|midnight/i, "moon"],
  [/emile zola|shakespeare|quill|literature|hamlet/i, "book"],
  [/rebecca|rose|miniver|gigi|terms of endearment|anora/i, "flower"],
  [/crown|king|emperor|patton|gandhi|man for all seasons/i, "crown"],
  [/greatest show|circus|tom jones|fair lady|west side story/i, "spotlight"],
  [/paris|american in paris|annie hall|french connection|crash/i, "city"],
  [/rain man|driving miss daisy|happened one night|cowboy/i, "road"],
  [/microphone|king's speech|speech/i, "microphone"],
  [/birdman|bird/i, "bird"],
  [/shape of water|coda|water|everything everywhere/i, "wave"],
  [/cimarron|green|valley/i, "wave"],
];

function getMotif(title: string, collectionSlug: string): Motif {
  const motifs =
    collectionSlug === "halloween-top-100" ? horrorMotifs : oscarMotifs;
  return (
    motifs.find(([pattern]) => pattern.test(title))?.[1] ??
    (collectionSlug === "halloween-top-100" ? "moon" : "spotlight")
  );
}

function titleLines(title: string) {
  const maxLength = title.length > 36 ? 17 : 21;
  const lines: string[] = [];
  let line = "";
  for (const word of title.split(/\s+/)) {
    if (line && `${line} ${word}`.length > maxLength) {
      lines.push(line);
      line = word;
    } else {
      line = line ? `${line} ${word}` : word;
    }
  }
  if (line) lines.push(line);
  return lines.length > 3
    ? [lines.slice(0, 2).join(" "), lines.slice(2).join(" ")]
    : lines;
}

function Artwork({ motif }: { motif: Motif }) {
  const ink = "#292820";
  const rust = "#ba522c";
  const brass = "#b9914e";
  const line = {
    fill: "none",
    stroke: ink,
    strokeWidth: 8,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };

  switch (motif) {
    case "bird":
      return (
        <g {...line}>
          <path
            d="M116 390c78-106 149-104 206-11 41-82 107-99 162-48-37 5-58 24-71 55 62-20 105-2 126 43-79-26-141-4-203 53-71-62-144-94-220-92Z"
            fill={ink}
          />
          <path
            d="M322 375c-12 43-34 76-66 99M355 372c18 37 43 65 77 84"
            stroke={brass}
          />
          <circle cx="423" cy="330" r="15" fill={rust} stroke="none" />
        </g>
      );
    case "book":
      return (
        <g {...line}>
          <path
            d="M122 266c70-28 135-17 200 33v236c-63-48-130-57-200-29V266Zm356 0c-70-28-135-17-200 33v236c63-48 130-57 200-29V266Z"
            fill="#f4f1ea"
          />
          <path
            d="M302 301v229M157 310c45-12 88-7 126 14m161-14c-45-12-88-7-126 14"
            stroke={rust}
          />
          <path
            d="M298 215c-43 34-44 76 2 110 46-34 45-76-2-110Z"
            fill={brass}
          />
        </g>
      );
    case "candle":
      return (
        <g {...line}>
          <path
            d="M300 205c-54 54-32 103 0 117 32-14 54-63 0-117Z"
            fill={brass}
            stroke={ink}
          />
          <path d="M300 322v-25m-77 38h154v230H223z" fill="#f4f1ea" />
          <path d="M236 365h128m-128 158h128" stroke={rust} />
          <path d="M183 590h234" />
        </g>
      );
    case "camera":
      return (
        <g {...line}>
          <path d="M151 300h298v211H151z" fill="#f4f1ea" />
          <path d="m194 300 30-53h117l31 53" fill={brass} />
          <circle cx="300" cy="405" r="75" fill={rust} />
          <circle cx="300" cy="405" r="39" fill="#f4f1ea" />
          <path d="M250 511v68m100-68v68m-128 0h156" />
        </g>
      );
    case "city":
      return (
        <g {...line}>
          <path
            d="M139 558V373h82v185m20 0V278h115v280m22 0V339h88v219"
            fill="#f4f1ea"
          />
          <path
            d="M174 411h14m-14 44h14m-14 44h14m95-186h18m-18 48h18m-18 48h18m-18 48h18m80-153h18m-18 48h18m-18 48h18"
            stroke={rust}
          />
          <path d="M108 558h384" />
          <circle cx="433" cy="237" r="35" fill={brass} stroke="none" />
        </g>
      );
    case "crown":
      return (
        <g {...line}>
          <path
            d="m153 360 86 67 61-157 62 157 85-67-36 190H189z"
            fill={brass}
          />
          <path d="M189 550h244m-219 47h194" />
          <circle cx="239" cy="426" r="14" fill={rust} stroke="none" />
          <circle cx="300" cy="309" r="14" fill={rust} stroke="none" />
          <circle cx="362" cy="426" r="14" fill={rust} stroke="none" />
        </g>
      );
    case "eye":
      return (
        <g {...line}>
          <path
            d="M109 402c105-133 277-133 382 0-105 133-277 133-382 0Z"
            fill="#f4f1ea"
          />
          <circle cx="300" cy="402" r="79" fill={rust} />
          <circle cx="300" cy="402" r="37" fill={ink} />
          <circle cx="323" cy="377" r="12" fill="#f4f1ea" stroke="none" />
          <path d="M300 283v-44m0 325v-44" stroke={brass} />
        </g>
      );
    case "flower":
      return (
        <g {...line}>
          <path d="M300 409v177m0-80c-55-57-91-44-95-3 34 23 65 21 95 3Zm0-42c53-57 89-44 93-3-33 23-64 21-93 3Z" />
          <g fill={rust} stroke={ink} strokeWidth="7">
            {[0, 60, 120, 180, 240, 300].map((rotation) => (
              <ellipse
                key={rotation}
                cx="300"
                cy="329"
                rx="35"
                ry="69"
                transform={`rotate(${rotation} 300 365)`}
              />
            ))}
          </g>
          <circle
            cx="300"
            cy="365"
            r="36"
            fill={brass}
            stroke={ink}
            strokeWidth="7"
          />
        </g>
      );
    case "globe":
      return (
        <g {...line}>
          <circle cx="300" cy="390" r="156" fill="#f4f1ea" />
          <path
            d="M144 390h312M300 234c-89 84-89 228 0 312m0-312c89 84 89 228 0 312M172 311h256m-256 158h256"
            stroke={rust}
          />
          <path
            d="M285 543c17 31 39 52 70 67m-121-12c-20 25-41 41-68 50"
            stroke={brass}
          />
        </g>
      );
    case "hotel":
      return (
        <g {...line}>
          <path d="M164 574V296h272v278M131 574h338" fill="#f4f1ea" />
          <path
            d="M204 350h33v40h-33zm79 0h34v40h-34zm79 0h34v40h-34zM204 433h33v40h-33zm79 0h34v40h-34zm79 0h34v40h-34z"
            fill={brass}
            stroke={ink}
            strokeWidth="6"
          />
          <path d="M272 574v-66h56v66" fill={rust} />
        </g>
      );
    case "house":
      return (
        <g {...line}>
          <path d="m133 366 167-139 167 139v200H133V366Z" fill="#f4f1ea" />
          <path
            d="M260 566V418h80v148m-159-142h48v52h-48zm190 0h48v52h-48z"
            fill={rust}
          />
          <path d="M96 577h408m-305 43h202" stroke={brass} />
          <path d="M300 227v-63m-31 32h62" />
        </g>
      );
    case "lightning":
      return (
        <g {...line}>
          <path d="m337 188-127 216h95l-39 179 128-243h-94z" fill={brass} />
          <path d="M134 578h332m-282 43h232" />
          <path
            d="M157 317c34-57 85-86 151-86m135 105c-33-52-78-80-136-84"
            stroke={rust}
          />
        </g>
      );
    case "mask":
      return (
        <g {...line}>
          <path
            d="M169 270c82-38 180-38 262 0v139c-28 94-73 155-131 184-58-29-103-90-131-184V270Z"
            fill="#f4f1ea"
          />
          <path
            d="M188 345c38-28 76-28 113 0-37 28-75 28-113 0Zm111 0c38-28 76-28 113 0-37 28-75 28-113 0Z"
            fill={ink}
            stroke="none"
          />
          <path d="M250 474c34 22 67 22 101 0" stroke={rust} />
          <path d="M300 271v277" stroke={brass} />
        </g>
      );
    case "microphone":
      return (
        <g {...line}>
          <rect x="257" y="208" width="86" height="212" rx="43" fill={brass} />
          <path d="M202 354v39c0 73 44 119 98 119s98-46 98-119v-39m-98 158v78m-79 0h158" />
          <path d="M277 255h46m-46 43h46m-46 43h46" stroke={rust} />
        </g>
      );
    case "mirror":
      return (
        <g {...line}>
          <path
            d="M300 211c97 0 143 83 143 190s-46 190-143 190-143-83-143-190 46-190 143-190Z"
            fill="#f4f1ea"
          />
          <path d="M300 224v354" stroke={rust} />
          <path
            d="M224 323c27-40 53-60 76-60m76 60c-27-40-53-60-76-60m-78 201c26 38 52 57 78 57m76-57c-26 38-52 57-76 57"
            stroke={brass}
          />
        </g>
      );
    case "moon":
      return (
        <g {...line}>
          <path
            d="M353 215c-111 14-165 112-142 217 23 106 119 159 218 122-81 90-222 73-283-26-63-103-26-241 81-297 40-21 83-26 126-16Z"
            fill={brass}
          />
          <path d="M137 574h326" />
          <circle cx="193" cy="299" r="9" fill={rust} stroke="none" />
          <circle cx="438" cy="352" r="7" fill={rust} stroke="none" />
          <circle cx="389" cy="241" r="6" fill={rust} stroke="none" />
        </g>
      );
    case "pod":
      return (
        <g {...line}>
          <path
            d="M300 207c65 68 103 146 103 220 0 85-45 145-103 145s-103-60-103-145c0-74 38-152 103-220Z"
            fill={brass}
          />
          <path
            d="M300 218v338m-74-120c44 31 104 31 148 0m-135-62c38 25 88 25 122 0"
            stroke={rust}
          />
          <path d="M149 576h302" />
        </g>
      );
    case "road":
      return (
        <g {...line}>
          <path d="M267 267h66l146 316H121l146-316Z" fill="#f4f1ea" />
          <path d="m267 267-94 316m160-316 94 316" />
          <path d="M300 321v48m0 38v55m0 43v51" stroke={rust} />
          <circle cx="300" cy="242" r="36" fill={brass} />
        </g>
      );
    case "ship":
      return (
        <g {...line}>
          <path d="m141 474 318 0-52 93H196l-55-93Z" fill={ink} />
          <path
            d="M292 244v230m22-202 112 202H314m-36-168-93 168h93"
            fill="#f4f1ea"
          />
          <path
            d="M112 589c41-22 82-22 123 0 43 22 85 22 127 0 43-22 85-22 127 0"
            stroke={rust}
          />
          <circle cx="407" cy="271" r="27" fill={brass} stroke="none" />
        </g>
      );
    case "spotlight":
      return (
        <g {...line}>
          <path
            d="M300 243 131 552h338L300 243Z"
            fill={brass}
            opacity=".55"
            stroke="none"
          />
          <path d="M300 214v44m-17-44h34m-184 358h334" />
          <path d="M192 552V364m216 188V364" stroke={rust} />
          <path d="M192 365c48 65 84 65 108 0 26 65 62 65 108 0" stroke={ink} />
          <circle cx="300" cy="525" r="27" fill={rust} stroke="none" />
        </g>
      );
    case "train":
      return (
        <g {...line}>
          <path d="M207 248h186v302H207z" fill="#f4f1ea" />
          <path d="M229 273h142v113H229z" fill={ink} />
          <circle cx="300" cy="445" r="41" fill={rust} />
          <path d="M185 550h230m-202 27h174m-147 26h120" />
          <path d="M169 274h38m186 0h38" stroke={brass} />
        </g>
      );
    case "wave":
      return (
        <g {...line}>
          <path
            d="M132 392c52-56 104-56 156 0 54-56 107-56 160 0m-316 72c52-56 104-56 156 0 54-56 107-56 160 0m-316 72c52-56 104-56 156 0 54-56 107-56 160 0"
            stroke={ink}
          />
          <circle cx="396" cy="266" r="48" fill={brass} stroke="none" />
          <path d="M168 310c35-29 72-37 109-24" stroke={rust} />
        </g>
      );
    case "wing":
      return (
        <g {...line}>
          <path
            d="M135 488c58-194 188-282 330-250-48 67-94 144-127 231 52-42 105-57 162-44-89 78-180 147-273 198-44-27-75-69-92-135Z"
            fill={ink}
          />
          <path
            d="M179 499c81-75 172-145 276-213M237 534c55-83 117-158 190-226M297 563c30-69 74-143 132-218"
            stroke={brass}
          />
          <circle cx="409" cy="274" r="15" fill={rust} stroke="none" />
        </g>
      );
  }
}

export function MoviePosterArtwork({
  movie,
  collectionSlug,
  watched,
}: {
  movie: Movie;
  collectionSlug: string;
  watched: boolean;
}) {
  const { t } = useTranslation();
  const title = movie.title;
  const lines = titleLines(title);
  const motif = getMotif(title, collectionSlug);
  const fontSize = lines.length === 1 ? 50 : lines.length === 2 ? 42 : 34;
  const lineHeight = fontSize + 5;
  const titleStart = 703 - ((lines.length - 1) * lineHeight) / 2;

  return (
    <svg
      viewBox="0 0 600 900"
      role="img"
      aria-label={`Poster: ${title}${movie.releaseYear ? ` (${movie.releaseYear})` : ""}`}
      className={`h-full w-full transition duration-500 group-hover:scale-[1.03] ${watched ? "" : "poster-unwatched"}`}
      data-poster-fallback="true"
    >
      <rect width="600" height="900" fill="#eee6d5" />
      <rect
        x="22"
        y="22"
        width="556"
        height="856"
        fill="none"
        stroke="#292820"
        strokeWidth="2"
      />
      <text
        x="300"
        y="77"
        textAnchor="middle"
        fontFamily="Arial, sans-serif"
        fontSize="16"
        letterSpacing="5"
        fill="#625e58"
      >
        {t("movie.posterLabel")}
      </text>
      <text
        x="300"
        y="117"
        textAnchor="middle"
        fontFamily="Arial, sans-serif"
        fontSize="18"
        letterSpacing="3"
        fill="#ba522c"
      >
        {movie.releaseYear ?? ""}
      </text>
      <Artwork motif={motif} />
      <path d="M95 635h410" stroke="#b9914e" strokeWidth="2" />
      <g
        fill="#292820"
        fontFamily="Georgia, serif"
        fontSize={fontSize}
        textAnchor="middle"
      >
        {lines.map((line, index) => (
          <text
            key={`${index}-${line}`}
            x="300"
            y={titleStart + index * lineHeight}
          >
            {line}
          </text>
        ))}
      </g>
    </svg>
  );
}
